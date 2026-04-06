package service_test

import (
	"bytes"
	"context"
	"database/sql"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"
)

type fakePaymentRepo struct {
	activeSubscriptions []model.Subscription
	activeSubByID       model.Subscription
	activeSubByIDErr    error

	createPaymentInput  model.Payment
	createPaymentOutput model.Payment
	createPaymentErr    error

	paymentByID         model.Payment
	paymentByIDErr      error
	paymentByUser       model.Payment
	paymentByUserErr    error
	userSubByPayment    model.UserSubscription
	userSubByPaymentErr error
	premiumCoverageEnd  *time.Time

	listAdminParams repository.PaymentListParams
	listAdminItems  []repository.AdminPaymentItem
	listAdminErr    error

	createdDocuments []model.Document
	createDocErr     error
	attachProofArgs  struct {
		paymentID  string
		userID     string
		documentID string
	}
	attachProofErr     error
	markSuccessParams  repository.MarkPaymentSuccessParams
	markSuccessErr     error
	markSuccessOutput  model.Payment
	markSuccessUserSub model.UserSubscription
	markFailedParams   repository.MarkPaymentFailedParams
	markFailedOutput   model.Payment
	markFailedErr      error
	markSentPaymentID  string
	markSentAt         time.Time
	notifications      []repository.PendingPaymentNotification
}

func (f *fakePaymentRepo) ListActiveSubscriptions(ctx context.Context) ([]model.Subscription, error) {
	return f.activeSubscriptions, nil
}

func (f *fakePaymentRepo) FindActiveSubscriptionByID(ctx context.Context, subscriptionID string) (model.Subscription, error) {
	if f.activeSubByIDErr != nil {
		return model.Subscription{}, f.activeSubByIDErr
	}
	return f.activeSubByID, nil
}

func (f *fakePaymentRepo) CreatePayment(ctx context.Context, payment model.Payment) (model.Payment, error) {
	f.createPaymentInput = payment
	if f.createPaymentErr != nil {
		return model.Payment{}, f.createPaymentErr
	}
	if f.createPaymentOutput.PaymentID != "" {
		return f.createPaymentOutput, nil
	}
	return payment, nil
}

func (f *fakePaymentRepo) CreateDocument(ctx context.Context, doc model.Document) (model.Document, error) {
	if f.createDocErr != nil {
		return model.Document{}, f.createDocErr
	}
	f.createdDocuments = append(f.createdDocuments, doc)
	return doc, nil
}

func (f *fakePaymentRepo) FindPaymentByID(ctx context.Context, paymentID string) (model.Payment, error) {
	if f.paymentByIDErr != nil {
		return model.Payment{}, f.paymentByIDErr
	}
	return f.paymentByID, nil
}

func (f *fakePaymentRepo) FindPaymentByIDAndUser(ctx context.Context, paymentID, userID string) (model.Payment, error) {
	if f.paymentByUserErr != nil {
		return model.Payment{}, f.paymentByUserErr
	}
	return f.paymentByUser, nil
}

func (f *fakePaymentRepo) FindUserSubscriptionByPaymentID(ctx context.Context, paymentID, userID string) (model.UserSubscription, error) {
	if f.userSubByPaymentErr != nil {
		return model.UserSubscription{}, f.userSubByPaymentErr
	}
	if f.userSubByPayment.UserSubscriptionID != "" {
		return f.userSubByPayment, nil
	}
	return model.UserSubscription{}, sql.ErrNoRows
}

func (f *fakePaymentRepo) FindPremiumCoverageEndAt(ctx context.Context, userID string, reference time.Time) (*time.Time, error) {
	return f.premiumCoverageEnd, nil
}

func (f *fakePaymentRepo) ListAdminPayments(ctx context.Context, params repository.PaymentListParams) ([]repository.AdminPaymentItem, error) {
	f.listAdminParams = params
	if f.listAdminErr != nil {
		return nil, f.listAdminErr
	}
	return f.listAdminItems, nil
}

func (f *fakePaymentRepo) ListPendingPaymentNotifications(ctx context.Context, limit int) ([]repository.PendingPaymentNotification, error) {
	return f.notifications, nil
}

func (f *fakePaymentRepo) AttachPaymentProofDocument(ctx context.Context, paymentID, userID, documentID string) error {
	f.attachProofArgs.paymentID = paymentID
	f.attachProofArgs.userID = userID
	f.attachProofArgs.documentID = documentID
	return f.attachProofErr
}

func (f *fakePaymentRepo) MarkPaymentNotificationSent(ctx context.Context, paymentID string, notifiedAt time.Time) error {
	f.markSentPaymentID = paymentID
	f.markSentAt = notifiedAt
	return nil
}

func (f *fakePaymentRepo) MarkPaymentSuccess(ctx context.Context, params repository.MarkPaymentSuccessParams) (model.Payment, model.UserSubscription, error) {
	f.markSuccessParams = params
	if f.markSuccessErr != nil {
		return model.Payment{}, model.UserSubscription{}, f.markSuccessErr
	}
	if f.markSuccessOutput.PaymentID != "" {
		return f.markSuccessOutput, f.markSuccessUserSub, nil
	}
	payment := f.paymentByID
	payment.Status = model.PaymentStatusSuccess
	payment.PaidAt = ptrTime(params.StartDate)
	payment.VerifiedBy = &params.VerifiedBy
	payment.VerifiedAt = ptrTime(params.StartDate)
	payment.UpdatedAt = params.StartDate

	subscription := model.UserSubscription{
		UserSubscriptionID:     "user-sub-1",
		UserID:                 payment.UserID,
		SubscriptionID:         payment.SubscriptionID,
		SourcePaymentID:        payment.PaymentID,
		PackageNameSnapshot:    payment.PackageNameSnapshot,
		DurationMonthsSnapshot: payment.DurationMonthsSnapshot,
		PriceAmountSnapshot:    payment.PriceAmountSnapshot,
		StartDate:              params.StartDate,
		EndDate:                params.StartDate.AddDate(0, payment.DurationMonthsSnapshot, 0),
		CreatedAt:              params.StartDate,
	}
	return payment, subscription, nil
}

func (f *fakePaymentRepo) MarkPaymentFailed(ctx context.Context, params repository.MarkPaymentFailedParams) (model.Payment, error) {
	f.markFailedParams = params
	if f.markFailedErr != nil {
		return model.Payment{}, f.markFailedErr
	}
	if f.markFailedOutput.PaymentID != "" {
		return f.markFailedOutput, nil
	}
	return model.Payment{PaymentID: params.PaymentID, Status: model.PaymentStatusFailed}, nil
}

type fakeDocumentStorage struct {
	stored service.StoredObject
}

func (f *fakeDocumentStorage) Upload(ctx context.Context, input service.UploadInput) (service.StoredObject, error) {
	return f.stored, nil
}

type fakePaymentSender struct {
	to      string
	subject string
	body    string
	err     error
}

func (f *fakePaymentSender) Send(ctx context.Context, to, subject, body string) error {
	f.to = to
	f.subject = subject
	f.body = body
	return f.err
}

func ptrTime(t time.Time) *time.Time {
	value := t.UTC()
	return &value
}

func makeProofHeader(t *testing.T) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	file, err := writer.CreateFormFile("file", "proof.pdf")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := file.Write([]byte("%PDF-1.7")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}

	files := req.MultipartForm.File["file"]
	if len(files) == 0 {
		t.Fatal("missing multipart file header")
	}
	return files[0]
}

func TestUpdatePaymentStatusUsesCoverageEndAtPurchaseTime(t *testing.T) {
	reference := time.Date(2026, time.April, 4, 10, 0, 0, 0, time.UTC)
	coverageEnd := time.Date(2026, time.April, 10, 10, 0, 0, 0, time.UTC)
	repo := &fakePaymentRepo{
		paymentByID: model.Payment{
			PaymentID:              "pay-1",
			TransactionID:          "PAY-20260404-100000-ABCDEF01",
			UserID:                 "user-1",
			SubscriptionID:         "sub-1",
			PackageNameSnapshot:    "The Scholar",
			DurationMonthsSnapshot: 3,
			PriceAmountSnapshot:    299000,
			Status:                 model.PaymentStatusPending,
			CreatedAt:              reference,
		},
		premiumCoverageEnd: &coverageEnd,
	}
	svc := service.NewPaymentServiceWithDeps(repo, &fakeDocumentStorage{})

	out, err := svc.UpdatePaymentStatus(context.Background(), "admin-1", "pay-1", string(model.PaymentStatusSuccess), nil, nil, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !repo.markSuccessParams.StartDate.Equal(coverageEnd) {
		t.Fatalf("expected start date %s, got %s", coverageEnd, repo.markSuccessParams.StartDate)
	}
	if out.PremiumActiveAt == nil || !out.PremiumActiveAt.Equal(coverageEnd) {
		t.Fatalf("expected premium active at %s, got %v", coverageEnd, out.PremiumActiveAt)
	}
}

func TestUploadProofForPaymentAttachesDocument(t *testing.T) {
	repo := &fakePaymentRepo{
		paymentByUser: model.Payment{
			PaymentID:     "pay-1",
			UserID:        "user-1",
			Status:        model.PaymentStatusPending,
			TransactionID: "TX-1",
		},
	}
	svc := service.NewPaymentServiceWithDeps(repo, &fakeDocumentStorage{stored: service.StoredObject{
		StoragePath: "uploads/user-1/payment_proof/proof.pdf",
		PublicURL:   "http://local/uploads/user-1/payment_proof/proof.pdf",
		SizeBytes:   8,
		MIMEType:    "application/pdf",
	}})

	header := makeProofHeader(t)
	doc, err := svc.UploadProofForPayment(context.Background(), "user-1", "pay-1", header)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if doc.DocumentType != model.DocumentTypePaymentProof {
		t.Fatalf("expected payment proof doc type, got %s", doc.DocumentType)
	}
	if repo.attachProofArgs.paymentID != "pay-1" || repo.attachProofArgs.userID != "user-1" {
		t.Fatalf("unexpected attach args: %+v", repo.attachProofArgs)
	}
}

func TestPaymentNotificationRunOnceMarksSent(t *testing.T) {
	repo := &fakePaymentRepo{
		notifications: []repository.PendingPaymentNotification{{
			PaymentID:        "pay-1",
			TransactionID:    "TX-1",
			UserID:           "user-1",
			UserName:         "Alice",
			UserEmail:        "alice@example.com",
			PackageName:      "The Scholar",
			Amount:           299000,
			ProofDocumentURL: "http://local/proof.pdf",
			CreatedAt:        time.Date(2026, time.April, 4, 10, 0, 0, 0, time.UTC),
		}},
	}
	sender := &fakePaymentSender{}
	svc := service.NewPaymentNotificationService(repo, sender, "admin@example.com")

	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if sender.to != "admin@example.com" {
		t.Fatalf("expected recipient admin@example.com, got %s", sender.to)
	}
	if repo.markSentPaymentID != "pay-1" {
		t.Fatalf("expected mark sent payment id pay-1, got %s", repo.markSentPaymentID)
	}
	if sender.subject == "" || sender.body == "" {
		t.Fatal("expected email content to be generated")
	}
}

func TestUploadProofForPaymentRejectsFinalizedPayment(t *testing.T) {
	repo := &fakePaymentRepo{
		paymentByUser: model.Payment{
			PaymentID: "pay-1",
			UserID:    "user-1",
			Status:    model.PaymentStatusSuccess,
		},
	}
	svc := service.NewPaymentServiceWithDeps(repo, &fakeDocumentStorage{stored: service.StoredObject{}})

	header := makeProofHeader(t)
	_, err := svc.UploadProofForPayment(context.Background(), "user-1", "pay-1", header)
	if err != errs.ErrPaymentNotPending {
		t.Fatalf("expected %v, got %v", errs.ErrPaymentNotPending, err)
	}
}

func TestNotificationServiceSkipsWhenNotConfigured(t *testing.T) {
	repo := &fakePaymentRepo{}
	svc := service.NewPaymentNotificationService(repo, &fakePaymentSender{}, "")
	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestCreatePaymentUsesSubscriptionSnapshotAndDefaultQris(t *testing.T) {
	t.Setenv("PAYMENT_QRIS_IMAGE_URL", "")
	repo := &fakePaymentRepo{
		activeSubByID: model.Subscription{
			SubscriptionID: "sub-1",
			Name:           "The Scholar",
			DurationMonths: 6,
			PriceAmount:    499000,
			Benefits:       []string{"A", "B"},
		},
	}

	svc := service.NewPaymentServiceWithDeps(repo, &fakeDocumentStorage{})
	out, err := svc.CreatePayment(context.Background(), "user-1", "sub-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Payment.PaymentID == "" || !strings.HasPrefix(out.Payment.TransactionID, "PAY-") {
		t.Fatalf("expected generated payment ids, got %+v", out.Payment)
	}
	if out.Payment.QrisImageURL != "-" {
		t.Fatalf("expected default qris '-', got %s", out.Payment.QrisImageURL)
	}
	if repo.createPaymentInput.SubscriptionID != "sub-1" || repo.createPaymentInput.UserID != "user-1" {
		t.Fatalf("unexpected create payment input: %+v", repo.createPaymentInput)
	}
	if repo.createPaymentInput.ExpiredAt == nil {
		t.Fatal("expected expired at to be set")
	}
	if len(repo.createPaymentInput.BenefitsSnapshot) != 2 {
		t.Fatalf("expected benefits copied, got %+v", repo.createPaymentInput.BenefitsSnapshot)
	}
}

func TestCreatePaymentRejectsBlankIdentifiers(t *testing.T) {
	repo := &fakePaymentRepo{}
	svc := service.NewPaymentServiceWithDeps(repo, &fakeDocumentStorage{})

	_, err := svc.CreatePayment(context.Background(), "", "sub-1")
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestGetMyPaymentBuildsPremiumWindowFromPaidAtFallback(t *testing.T) {
	paidAt := time.Date(2026, time.April, 5, 8, 30, 0, 0, time.UTC)
	expires := paidAt.AddDate(0, 2, 0)
	repo := &fakePaymentRepo{
		paymentByUser: model.Payment{
			PaymentID:              "pay-1",
			UserID:                 "user-1",
			Status:                 model.PaymentStatusSuccess,
			DurationMonthsSnapshot: 2,
			PaidAt:                 &paidAt,
		},
		userSubByPaymentErr: sql.ErrNoRows,
	}
	svc := service.NewPaymentServiceWithDeps(repo, &fakeDocumentStorage{})

	out, err := svc.GetMyPayment(context.Background(), "user-1", "pay-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.PremiumActiveAt == nil || !out.PremiumActiveAt.Equal(paidAt) {
		t.Fatalf("expected premium active at %s, got %v", paidAt, out.PremiumActiveAt)
	}
	if out.PremiumExpiredAt == nil || !out.PremiumExpiredAt.Equal(expires) {
		t.Fatalf("expected premium expired at %s, got %v", expires, out.PremiumExpiredAt)
	}
}

func TestListAdminPaymentsNormalizesPagination(t *testing.T) {
	repo := &fakePaymentRepo{}
	svc := service.NewPaymentServiceWithDeps(repo, &fakeDocumentStorage{})

	_, err := svc.ListAdminPayments(context.Background(), "alice", "pending", -1, 500)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.listAdminParams.Limit != 100 {
		t.Fatalf("expected normalized limit 100, got %d", repo.listAdminParams.Limit)
	}
	if repo.listAdminParams.Offset != 0 {
		t.Fatalf("expected offset 0, got %d", repo.listAdminParams.Offset)
	}
	if repo.listAdminParams.Status != model.PaymentStatusPending {
		t.Fatalf("expected status pending, got %s", repo.listAdminParams.Status)
	}
	if repo.listAdminParams.Query != "alice" {
		t.Fatalf("expected query alice, got %s", repo.listAdminParams.Query)
	}
}

func TestUpdatePaymentStatusFailedTrimsOptionalValues(t *testing.T) {
	repo := &fakePaymentRepo{}
	svc := service.NewPaymentServiceWithDeps(repo, &fakeDocumentStorage{})
	note := "  duplicate transfer  "
	proof := "   "

	_, err := svc.UpdatePaymentStatus(
		context.Background(),
		"admin-1",
		"pay-1",
		string(model.PaymentStatusFailed),
		nil,
		&note,
		&proof,
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.markFailedParams.AdminNote == nil || *repo.markFailedParams.AdminNote != "duplicate transfer" {
		t.Fatalf("expected trimmed admin note, got %+v", repo.markFailedParams.AdminNote)
	}
	if repo.markFailedParams.ProofDocumentID != nil {
		t.Fatalf("expected empty proof id to become nil, got %+v", repo.markFailedParams.ProofDocumentID)
	}
}

func TestBuildPaymentInstructionIncludesTransactionContext(t *testing.T) {
	instruction := service.BuildPaymentInstruction("TX-123", 299000, "IDR", "+62812")
	if !strings.Contains(instruction, "TX-123") || !strings.Contains(instruction, "+62812") {
		t.Fatalf("expected instruction to include transaction and whatsapp, got %s", instruction)
	}
}

func TestMain(m *testing.M) {
	os.Setenv("DOCUMENT_STORAGE_PROVIDER", "local")
	os.Setenv("DOCUMENT_STORAGE_DIR", os.TempDir())
	os.Exit(m.Run())
}
