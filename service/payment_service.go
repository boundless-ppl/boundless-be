package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"strings"
	"time"

	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"

	"github.com/google/uuid"
)

const (
	defaultPaymentChannel = "qris_manual"
	paymentDiscountPct    = int64(90)
)

type PaymentService struct {
	repo    repository.PaymentRepository
	storage DocumentStorage
}

type CreatePaymentOutput struct {
	Payment model.Payment
}

type PaymentDetailOutput struct {
	Payment          model.Payment
	PremiumActiveAt  *time.Time
	PremiumExpiredAt *time.Time
}

type AdminListPaymentsOutput struct {
	Items []repository.AdminPaymentItem
}

type AdminUpdatePaymentStatusOutput struct {
	Payment          model.Payment
	PremiumActiveAt  *time.Time
	PremiumExpiredAt *time.Time
}

func NewPaymentService(repo repository.PaymentRepository) *PaymentService {
	return &PaymentService{
		repo:    repo,
		storage: mustBuildDocumentStorage(),
	}
}

func NewPaymentServiceWithDeps(repo repository.PaymentRepository, storage DocumentStorage) *PaymentService {
	if storage == nil {
		storage = mustBuildDocumentStorage()
	}
	return &PaymentService{
		repo:    repo,
		storage: storage,
	}
}

func (s *PaymentService) ListPackages(ctx context.Context) ([]model.Subscription, error) {
	packages, err := s.repo.ListActiveSubscriptions(ctx)
	if err != nil {
		return nil, err
	}

	for i := range packages {
		packages[i].PriceAmount = applyPaymentDiscount(packages[i].PriceAmount)
	}

	return packages, nil
}

func (s *PaymentService) CreatePayment(ctx context.Context, userID, subscriptionID string) (CreatePaymentOutput, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(subscriptionID) == "" {
		return CreatePaymentOutput{}, errs.ErrInvalidInput
	}

	subscription, err := s.repo.FindActiveSubscriptionByID(ctx, subscriptionID)
	if err != nil {
		return CreatePaymentOutput{}, err
	}

	now := time.Now().UTC()
	expiredAt := now.Add(24 * time.Hour)
	_, discountedAmount := PaymentPriceBreakdownFromOriginal(subscription.PriceAmount)
	payment := model.Payment{
		PaymentID:              uuid.NewString(),
		TransactionID:          generateTransactionID(now),
		UserID:                 userID,
		SubscriptionID:         subscription.SubscriptionID,
		PackageNameSnapshot:    subscription.Name,
		DurationMonthsSnapshot: subscription.DurationMonths,
		PriceAmountSnapshot:    discountedAmount,
		BenefitsSnapshot:       cloneBenefits(subscription.Benefits),
		PaymentChannel:         defaultPaymentChannel,
		QrisImageURL:           strings.TrimSpace(os.Getenv("PAYMENT_QRIS_IMAGE_URL")),
		Status:                 model.PaymentStatusPending,
		ExpiredAt:              &expiredAt,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if payment.QrisImageURL == "" {
		payment.QrisImageURL = "-"
	}

	created, err := s.repo.CreatePayment(ctx, payment)
	if err != nil {
		return CreatePaymentOutput{}, err
	}

	return CreatePaymentOutput{Payment: created}, nil
}

func (s *PaymentService) GetMyPayment(ctx context.Context, userID, paymentID string) (PaymentDetailOutput, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(paymentID) == "" {
		return PaymentDetailOutput{}, errs.ErrInvalidInput
	}

	payment, err := s.repo.FindPaymentByIDAndUser(ctx, paymentID, userID)
	if err != nil {
		return PaymentDetailOutput{}, err
	}

	output := PaymentDetailOutput{Payment: payment}
	if payment.Status == model.PaymentStatusSuccess {
		userSub, err := s.repo.FindUserSubscriptionByPaymentID(ctx, payment.PaymentID, userID)
		if err == nil {
			activeAt := userSub.StartDate.UTC()
			expiredAt := userSub.EndDate.UTC()
			output.PremiumActiveAt = &activeAt
			output.PremiumExpiredAt = &expiredAt
		} else if !errors.Is(err, sql.ErrNoRows) {
			return PaymentDetailOutput{}, err
		} else if payment.PaidAt != nil {
			activeAt := payment.PaidAt.UTC()
			expiredAt := activeAt.AddDate(0, payment.DurationMonthsSnapshot, 0)
			output.PremiumActiveAt = &activeAt
			output.PremiumExpiredAt = &expiredAt
		}
	}

	return output, nil
}

func (s *PaymentService) ListAdminPayments(ctx context.Context, query, status string, page, pageSize int) (AdminListPaymentsOutput, error) {
	statusValue, err := parseOptionalStatus(status)
	if err != nil {
		return AdminListPaymentsOutput{}, err
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	items, err := s.repo.ListAdminPayments(ctx, repository.PaymentListParams{
		Query:  strings.TrimSpace(query),
		Status: statusValue,
		Since:  time.Now().UTC().AddDate(-1, 0, 0),
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	})
	if err != nil {
		return AdminListPaymentsOutput{}, err
	}

	return AdminListPaymentsOutput{Items: items}, nil
}

func (s *PaymentService) UpdatePaymentStatus(
	ctx context.Context,
	adminUserID string,
	paymentID string,
	status string,
	startDate *string,
	adminNote *string,
	proofDocumentID *string,
) (AdminUpdatePaymentStatusOutput, error) {
	if strings.TrimSpace(adminUserID) == "" || strings.TrimSpace(paymentID) == "" {
		return AdminUpdatePaymentStatusOutput{}, errs.ErrInvalidInput
	}

	statusValue, err := parseRequiredStatus(status)
	if err != nil {
		return AdminUpdatePaymentStatusOutput{}, err
	}

	normalizedNote := trimPtr(adminNote)
	normalizedProofID := trimPtr(proofDocumentID)

	switch statusValue {
	case model.PaymentStatusSuccess:
		startAt, err := s.resolvePaymentStartDate(ctx, paymentID, startDate)
		if err != nil {
			return AdminUpdatePaymentStatusOutput{}, err
		}

		payment, userSub, err := s.repo.MarkPaymentSuccess(ctx, repository.MarkPaymentSuccessParams{
			PaymentID:       paymentID,
			VerifiedBy:      adminUserID,
			StartDate:       startAt,
			AdminNote:       normalizedNote,
			ProofDocumentID: normalizedProofID,
		})
		if err != nil {
			return AdminUpdatePaymentStatusOutput{}, err
		}

		activeAt := userSub.StartDate.UTC()
		expiredAt := userSub.EndDate.UTC()
		return AdminUpdatePaymentStatusOutput{
			Payment:          payment,
			PremiumActiveAt:  &activeAt,
			PremiumExpiredAt: &expiredAt,
		}, nil

	case model.PaymentStatusFailed:
		payment, err := s.repo.MarkPaymentFailed(ctx, repository.MarkPaymentFailedParams{
			PaymentID:       paymentID,
			VerifiedBy:      adminUserID,
			AdminNote:       normalizedNote,
			ProofDocumentID: normalizedProofID,
		})
		if err != nil {
			return AdminUpdatePaymentStatusOutput{}, err
		}
		return AdminUpdatePaymentStatusOutput{Payment: payment}, nil
	}

	return AdminUpdatePaymentStatusOutput{}, errs.ErrInvalidPaymentStatus
}

func (s *PaymentService) UploadProofForPayment(ctx context.Context, userID, paymentID string, file *multipart.FileHeader) (model.Document, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(paymentID) == "" {
		return model.Document{}, errs.ErrInvalidInput
	}
	if err := validateUploadHeader(file); err != nil {
		return model.Document{}, err
	}

	payment, err := s.repo.FindPaymentByIDAndUser(ctx, paymentID, userID)
	if err != nil {
		return model.Document{}, err
	}
	if payment.Status != model.PaymentStatusPending {
		return model.Document{}, errs.ErrPaymentNotPending
	}

	stored, err := s.storage.Upload(ctx, UploadInput{
		UserID:       userID,
		DocumentType: model.DocumentTypePaymentProof,
		Header:       file,
	})
	if err != nil {
		return model.Document{}, err
	}

	doc := model.Document{
		DocumentID:       uuid.NewString(),
		UserID:           payment.UserID,
		OriginalFilename: file.Filename,
		StoragePath:      stored.StoragePath,
		PublicURL:        stored.PublicURL,
		MIMEType:         stored.MIMEType,
		SizeBytes:        stored.SizeBytes,
		DocumentType:     model.DocumentTypePaymentProof,
		UploadedAt:       time.Now().UTC(),
	}

	created, err := s.repo.CreateDocument(ctx, doc)
	if err != nil {
		return model.Document{}, err
	}

	if err := s.repo.AttachPaymentProofDocument(ctx, paymentID, userID, created.DocumentID); err != nil {
		return model.Document{}, err
	}

	return created, nil
}

func (s *PaymentService) resolvePaymentStartDate(ctx context.Context, paymentID string, requestedStartDate *string) (time.Time, error) {
	if requestedStartDate != nil && strings.TrimSpace(*requestedStartDate) != "" {
		return parseStartDate(requestedStartDate)
	}

	payment, err := s.repo.FindPaymentByID(ctx, paymentID)
	if err != nil {
		return time.Time{}, err
	}

	coverageEnd, err := s.repo.FindPremiumCoverageEndAt(ctx, payment.UserID, payment.CreatedAt.UTC())
	if err != nil {
		return time.Time{}, err
	}
	if coverageEnd != nil {
		return coverageEnd.UTC(), nil
	}

	return time.Now().UTC(), nil
}

func BuildPaymentInstruction(transactionID string, amount int64, currency, whatsappNumber string) string {
	return fmt.Sprintf(
		"1) Transfer sesuai nominal %s %d via QRIS. 2) Simpan bukti transfer. 3) Konfirmasi ke admin via WhatsApp %s dengan mencantumkan Transaction ID %s.",
		currency,
		amount,
		whatsappNumber,
		transactionID,
	)
}

func parseOptionalStatus(value string) (model.PaymentStatus, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return model.PaymentStatus(""), nil
	}

	return parseRequiredStatus(trimmed)
}

func parseRequiredStatus(value string) (model.PaymentStatus, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	switch model.PaymentStatus(trimmed) {
	case model.PaymentStatusPending, model.PaymentStatusSuccess, model.PaymentStatusFailed:
		return model.PaymentStatus(trimmed), nil
	default:
		return model.PaymentStatus(""), errs.ErrInvalidPaymentStatus
	}
}

func parseStartDate(startDate *string) (time.Time, error) {
	if startDate == nil || strings.TrimSpace(*startDate) == "" {
		return time.Now().UTC(), nil
	}

	trimmed := strings.TrimSpace(*startDate)
	if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", trimmed); err == nil {
		return t.UTC(), nil
	}

	return time.Time{}, errs.ErrInvalidInput
}

func trimPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func generateTransactionID(now time.Time) string {
	randomSuffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	return "PAY-" + now.UTC().Format("20060102-150405") + "-" + strings.ToUpper(randomSuffix)
}

func cloneBenefits(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func applyPaymentDiscount(amount int64) int64 {
	if amount <= 0 {
		return 0
	}
	discount := amount * paymentDiscountPct / 100
	finalAmount := amount - discount
	if finalAmount < 0 {
		return 0
	}
	return finalAmount
}

func PaymentPriceBreakdownFromOriginal(originalAmount int64) (int64, int64) {
	if originalAmount <= 0 {
		return 0, 0
	}
	return originalAmount, applyPaymentDiscount(originalAmount)
}

func PaymentPriceBreakdownFromDiscounted(discountedAmount int64) (int64, int64) {
	if discountedAmount <= 0 {
		return 0, 0
	}
	payablePct := int64(100) - paymentDiscountPct
	if payablePct <= 0 {
		return 0, discountedAmount
	}
	originalAmount := discountedAmount * 100 / payablePct
	if originalAmount < discountedAmount {
		originalAmount = discountedAmount
	}
	return originalAmount, discountedAmount
}
