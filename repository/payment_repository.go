package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"boundless-be/errs"
	"boundless-be/model"

	"github.com/google/uuid"
)

type PaymentListParams struct {
	Query  string
	Status model.PaymentStatus
	Since  time.Time
	Limit  int
	Offset int
}

type AdminPaymentItem struct {
	PaymentID        string
	TransactionID    string
	UserID           string
	UserName         string
	PackageName      string
	Amount           int64
	NormalAmount     int64
	Status           model.PaymentStatus
	TransactionAt    time.Time
	ProofDocumentID  *string
	ProofDocumentURL *string
}

type PendingPaymentNotification struct {
	PaymentID        string
	TransactionID    string
	UserID           string
	UserName         string
	UserEmail        string
	PackageName      string
	Amount           int64
	ProofDocumentURL string
	CreatedAt        time.Time
}

type MarkPaymentSuccessParams struct {
	PaymentID       string
	VerifiedBy      string
	StartDate       time.Time
	AdminNote       *string
	ProofDocumentID *string
}

type MarkPaymentFailedParams struct {
	PaymentID       string
	VerifiedBy      string
	AdminNote       *string
	ProofDocumentID *string
}

type PaymentRepository interface {
	ListActiveSubscriptions(ctx context.Context) ([]model.Subscription, error)
	FindActiveSubscriptionByID(ctx context.Context, subscriptionID string) (model.Subscription, error)
	CreatePayment(ctx context.Context, payment model.Payment) (model.Payment, error)
	CreateDocument(ctx context.Context, doc model.Document) (model.Document, error)
	FindPaymentByID(ctx context.Context, paymentID string) (model.Payment, error)
	FindPaymentByIDAndUser(ctx context.Context, paymentID, userID string) (model.Payment, error)
	FindUserSubscriptionByPaymentID(ctx context.Context, paymentID, userID string) (model.UserSubscription, error)
	FindPremiumCoverageEndAt(ctx context.Context, userID string, reference time.Time) (*time.Time, error)
	FindCurrentPremiumSubscription(ctx context.Context, userID string, reference time.Time) (model.UserSubscription, error)
	ListAdminPayments(ctx context.Context, params PaymentListParams) ([]AdminPaymentItem, error)
	ListPendingPaymentNotifications(ctx context.Context, limit int) ([]PendingPaymentNotification, error)
	AttachPaymentProofDocument(ctx context.Context, paymentID, userID, documentID string) error
	MarkPaymentNotificationSent(ctx context.Context, paymentID string, notifiedAt time.Time) error
	MarkPaymentSuccess(ctx context.Context, params MarkPaymentSuccessParams) (model.Payment, model.UserSubscription, error)
	MarkPaymentFailed(ctx context.Context, params MarkPaymentFailedParams) (model.Payment, error)
}

type DBPaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) *DBPaymentRepository {
	return &DBPaymentRepository{db: db}
}

func (r *DBPaymentRepository) ListActiveSubscriptions(ctx context.Context) ([]model.Subscription, error) {
	query := `
		SELECT subscription_id, package_key, name, description, duration_months, price_amount,
		       benefits_json, is_active, created_at, updated_at, normal_price_amount, discount_price_amount
		FROM subscriptions
		WHERE is_active = TRUE
		ORDER BY duration_months ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list active subscriptions: %w", err)
	}
	defer rows.Close()

	subscriptions := make([]model.Subscription, 0)
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscriptions: %w", err)
	}

	return subscriptions, nil
}

func (r *DBPaymentRepository) FindActiveSubscriptionByID(ctx context.Context, subscriptionID string) (model.Subscription, error) {
	query := `
		SELECT subscription_id, package_key, name, description, duration_months, price_amount,
		       benefits_json, is_active, created_at, updated_at, normal_price_amount, discount_price_amount
		FROM subscriptions
		WHERE subscription_id = $1 AND is_active = TRUE
	`

	row := r.db.QueryRowContext(ctx, query, subscriptionID)
	sub, err := scanSubscription(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Subscription{}, errs.ErrSubscriptionNotFound
		}
		return model.Subscription{}, err
	}

	return sub, nil
}

func (r *DBPaymentRepository) CreatePayment(ctx context.Context, payment model.Payment) (model.Payment, error) {
	benefitsJSON, err := json.Marshal(payment.BenefitsSnapshot)
	if err != nil {
		return model.Payment{}, fmt.Errorf("marshal payment benefits: %w", err)
	}

	query := `
		INSERT INTO payments (
			payment_id, transaction_id, user_id, subscription_id, package_name_snapshot, duration_months_snapshot,
			price_amount_snapshot, normal_price_snapshot, discount_price_snapshot, benefits_snapshot_json, payment_channel, qris_image_url,
			status, admin_notified_at, expired_at, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	`

	_, err = r.db.ExecContext(
		ctx,
		query,
		payment.PaymentID,
		payment.TransactionID,
		payment.UserID,
		payment.SubscriptionID,
		payment.PackageNameSnapshot,
		payment.DurationMonthsSnapshot,
		payment.PriceAmountSnapshot,
		payment.NormalPriceSnapshot,
		payment.DiscountPriceSnapshot,
		benefitsJSON,
		payment.PaymentChannel,
		payment.QrisImageURL,
		payment.Status,
		nil,
		payment.ExpiredAt,
		payment.CreatedAt,
		payment.UpdatedAt,
	)
	if err != nil {
		return model.Payment{}, fmt.Errorf("insert payment: %w", err)
	}

	return payment, nil
}

func (r *DBPaymentRepository) CreateDocument(ctx context.Context, doc model.Document) (model.Document, error) {
	query := `
		INSERT INTO documents
		(document_id, user_id, original_filename, storage_path, public_url, mime_type, size_bytes, document_type, uploaded_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		doc.DocumentID,
		doc.UserID,
		doc.OriginalFilename,
		doc.StoragePath,
		doc.PublicURL,
		doc.MIMEType,
		doc.SizeBytes,
		doc.DocumentType,
		doc.UploadedAt,
	)
	if err != nil {
		return model.Document{}, fmt.Errorf("insert payment proof document: %w", err)
	}

	return doc, nil
}

func (r *DBPaymentRepository) FindPaymentByID(ctx context.Context, paymentID string) (model.Payment, error) {
	query := `
		SELECT payment_id, transaction_id, user_id, subscription_id, package_name_snapshot, duration_months_snapshot,
		       price_amount_snapshot, normal_price_snapshot, discount_price_snapshot, benefits_snapshot_json, payment_channel, qris_image_url,
		       status, admin_note, proof_document_id, verified_by, verified_at, paid_at, expired_at, created_at, updated_at
		FROM payments
		WHERE payment_id = $1
	`

	payment, err := r.scanPayment(r.db.QueryRowContext(ctx, query, paymentID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Payment{}, errs.ErrPaymentNotFound
		}
		return model.Payment{}, err
	}

	return payment, nil
}

func (r *DBPaymentRepository) FindPaymentByIDAndUser(ctx context.Context, paymentID, userID string) (model.Payment, error) {
	query := `
		SELECT payment_id, transaction_id, user_id, subscription_id, package_name_snapshot, duration_months_snapshot,
		       price_amount_snapshot, normal_price_snapshot, discount_price_snapshot, benefits_snapshot_json, payment_channel, qris_image_url,
		       status, admin_note, proof_document_id, verified_by, verified_at, paid_at, expired_at, created_at, updated_at
		FROM payments
		WHERE payment_id = $1 AND user_id = $2
	`

	payment, err := r.scanPayment(r.db.QueryRowContext(ctx, query, paymentID, userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Payment{}, errs.ErrPaymentNotFound
		}
		return model.Payment{}, err
	}

	return payment, nil
}

func (r *DBPaymentRepository) FindUserSubscriptionByPaymentID(ctx context.Context, paymentID, userID string) (model.UserSubscription, error) {
	query := `
		SELECT user_subscription_id, user_id, subscription_id, source_payment_id,
		       package_name_snapshot, duration_months_snapshot, price_amount_snapshot,
		       start_date, end_date, created_at
		FROM user_subscriptions
		WHERE source_payment_id = $1 AND user_id = $2
		LIMIT 1
	`

	var sub model.UserSubscription
	err := r.db.QueryRowContext(ctx, query, paymentID, userID).Scan(
		&sub.UserSubscriptionID,
		&sub.UserID,
		&sub.SubscriptionID,
		&sub.SourcePaymentID,
		&sub.PackageNameSnapshot,
		&sub.DurationMonthsSnapshot,
		&sub.PriceAmountSnapshot,
		&sub.StartDate,
		&sub.EndDate,
		&sub.CreatedAt,
	)
	if err != nil {
		return model.UserSubscription{}, err
	}

	return sub, nil
}

func (r *DBPaymentRepository) FindPremiumCoverageEndAt(ctx context.Context, userID string, reference time.Time) (*time.Time, error) {
	query := `
		SELECT MAX(end_date)
		FROM user_subscriptions
		WHERE user_id = $1
		  AND start_date <= $2
		  AND end_date > $2
	`

	var endAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, userID, reference).Scan(&endAt); err != nil {
		return nil, fmt.Errorf("find premium coverage end date: %w", err)
	}
	if !endAt.Valid {
		return nil, nil
	}

	value := endAt.Time.UTC()
	return &value, nil
}

func (r *DBPaymentRepository) FindCurrentPremiumSubscription(ctx context.Context, userID string, reference time.Time) (model.UserSubscription, error) {
	query := `
		SELECT user_subscription_id, user_id, subscription_id, source_payment_id,
		       package_name_snapshot, duration_months_snapshot, price_amount_snapshot,
		       start_date, end_date, created_at
		FROM user_subscriptions
		WHERE user_id = $1
		ORDER BY start_date ASC, end_date ASC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return model.UserSubscription{}, fmt.Errorf("find current premium subscription: %w", err)
	}
	defer rows.Close()

	var (
		window    model.UserSubscription
		hasWindow bool
		current   model.UserSubscription
		prevEnd   time.Time
	)

	for rows.Next() {
		var sub model.UserSubscription
		if err := rows.Scan(
			&sub.UserSubscriptionID,
			&sub.UserID,
			&sub.SubscriptionID,
			&sub.SourcePaymentID,
			&sub.PackageNameSnapshot,
			&sub.DurationMonthsSnapshot,
			&sub.PriceAmountSnapshot,
			&sub.StartDate,
			&sub.EndDate,
			&sub.CreatedAt,
		); err != nil {
			return model.UserSubscription{}, fmt.Errorf("scan premium subscription: %w", err)
		}

		sub.StartDate = sub.StartDate.UTC()
		sub.EndDate = sub.EndDate.UTC()

		if !hasWindow {
			current = sub
			prevEnd = sub.EndDate
			hasWindow = true
			continue
		}

		if !sub.StartDate.After(prevEnd) {
			if sub.EndDate.After(prevEnd) {
				current.EndDate = sub.EndDate
				prevEnd = sub.EndDate
			}
			continue
		}

		if !reference.Before(current.StartDate) && !reference.After(current.EndDate) {
			window = current
			break
		}

		current = sub
		prevEnd = sub.EndDate
	}

	if err := rows.Err(); err != nil {
		return model.UserSubscription{}, fmt.Errorf("iterate premium subscriptions: %w", err)
	}

	if hasWindow && window.UserSubscriptionID == "" {
		if !reference.Before(current.StartDate) && !reference.After(current.EndDate) {
			window = current
		}
	}

	if window.UserSubscriptionID == "" {
		return model.UserSubscription{}, errs.ErrPremiumSubscriptionNotFound
	}

	return window, nil
}

func (r *DBPaymentRepository) ListAdminPayments(ctx context.Context, params PaymentListParams) ([]AdminPaymentItem, error) {
	query := `
		SELECT p.payment_id, p.transaction_id, p.user_id, u.nama_lengkap,
		       p.package_name_snapshot, p.price_amount_snapshot,
		       COALESCE(p.normal_price_snapshot, p.price_amount_snapshot * 10, p.price_amount_snapshot) AS normal_amount,
		       p.status, p.created_at,
		       p.proof_document_id, d.public_url
		FROM payments p
		JOIN users u ON u.user_id = p.user_id
		LEFT JOIN documents d ON d.document_id = p.proof_document_id
		WHERE p.created_at >= $1
		  AND ($2 = '' OR p.status = $2)
		  AND (
			$3 = ''
			OR LOWER(u.nama_lengkap) LIKE '%' || LOWER($3) || '%'
			OR LOWER(p.transaction_id) LIKE '%' || LOWER($3) || '%'
		  )
		ORDER BY p.created_at DESC
		LIMIT $4 OFFSET $5
	`

	rows, err := r.db.QueryContext(ctx, query, params.Since, params.Status, strings.TrimSpace(params.Query), params.Limit, params.Offset)
	if err != nil {
		return nil, fmt.Errorf("list admin payments: %w", err)
	}
	defer rows.Close()

	items := make([]AdminPaymentItem, 0)
	for rows.Next() {
		var item AdminPaymentItem
		var proofDocumentID sql.NullString
		var proofDocumentURL sql.NullString
		if err := rows.Scan(
			&item.PaymentID,
			&item.TransactionID,
			&item.UserID,
			&item.UserName,
			&item.PackageName,
			&item.Amount,
			&item.NormalAmount,
			&item.Status,
			&item.TransactionAt,
			&proofDocumentID,
			&proofDocumentURL,
		); err != nil {
			return nil, fmt.Errorf("scan admin payment: %w", err)
		}
		if proofDocumentID.Valid {
			value := proofDocumentID.String
			item.ProofDocumentID = &value
		}
		if proofDocumentURL.Valid {
			value := proofDocumentURL.String
			item.ProofDocumentURL = &value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin payments: %w", err)
	}

	return items, nil
}

func (r *DBPaymentRepository) ListPendingPaymentNotifications(ctx context.Context, limit int) ([]PendingPaymentNotification, error) {
	if limit < 1 {
		limit = 20
	}

	query := `
		SELECT p.payment_id, p.transaction_id, p.user_id, u.nama_lengkap, u.email,
		       p.package_name_snapshot, p.price_amount_snapshot, d.public_url, p.created_at
		FROM payments p
		JOIN users u ON u.user_id = p.user_id
		LEFT JOIN documents d ON d.document_id = p.proof_document_id
		WHERE p.status = $1
		  AND p.proof_document_id IS NOT NULL
		  AND p.admin_notified_at IS NULL
		ORDER BY p.created_at ASC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, model.PaymentStatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending payment notifications: %w", err)
	}
	defer rows.Close()

	items := make([]PendingPaymentNotification, 0)
	for rows.Next() {
		var item PendingPaymentNotification
		var proofDocumentURL sql.NullString
		if err := rows.Scan(
			&item.PaymentID,
			&item.TransactionID,
			&item.UserID,
			&item.UserName,
			&item.UserEmail,
			&item.PackageName,
			&item.Amount,
			&proofDocumentURL,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan pending payment notification: %w", err)
		}
		if proofDocumentURL.Valid {
			item.ProofDocumentURL = proofDocumentURL.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending payment notifications: %w", err)
	}

	return items, nil
}

func (r *DBPaymentRepository) AttachPaymentProofDocument(ctx context.Context, paymentID, userID, documentID string) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE payments
		SET proof_document_id = $3,
		    updated_at = $4
		WHERE payment_id = $1
		  AND user_id = $2
		  AND status = $5
	`, paymentID, userID, documentID, now, model.PaymentStatusPending)
	if err != nil {
		return fmt.Errorf("attach payment proof document: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows attach proof: %w", err)
	}
	if affected == 0 {
		existingStatus := model.PaymentStatus("")
		checkErr := r.db.QueryRowContext(ctx, `SELECT status FROM payments WHERE payment_id = $1 AND user_id = $2`, paymentID, userID).Scan(&existingStatus)
		if checkErr != nil {
			if errors.Is(checkErr, sql.ErrNoRows) {
				return errs.ErrPaymentNotFound
			}
			return fmt.Errorf("check payment status before attach proof: %w", checkErr)
		}
		return errs.ErrPaymentNotPending
	}
	return nil
}

func (r *DBPaymentRepository) MarkPaymentNotificationSent(ctx context.Context, paymentID string, notifiedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE payments
		SET admin_notified_at = $2,
		    updated_at = $2
		WHERE payment_id = $1
		  AND admin_notified_at IS NULL
	`, paymentID, notifiedAt.UTC())
	if err != nil {
		return fmt.Errorf("mark payment notification sent: %w", err)
	}
	return nil
}

func (r *DBPaymentRepository) MarkPaymentSuccess(ctx context.Context, params MarkPaymentSuccessParams) (model.Payment, model.UserSubscription, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Payment{}, model.UserSubscription{}, fmt.Errorf("begin tx mark payment success: %w", err)
	}
	defer tx.Rollback()

	payment, err := r.scanPayment(
		tx.QueryRowContext(ctx, `
			SELECT payment_id, transaction_id, user_id, subscription_id, package_name_snapshot, duration_months_snapshot,
			       price_amount_snapshot, normal_price_snapshot, discount_price_snapshot, benefits_snapshot_json, payment_channel, qris_image_url,
			       status, admin_note, proof_document_id, verified_by, verified_at, paid_at, expired_at, created_at, updated_at
			FROM payments
			WHERE payment_id = $1
			FOR UPDATE
		`, params.PaymentID),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Payment{}, model.UserSubscription{}, errs.ErrPaymentNotFound
		}
		return model.Payment{}, model.UserSubscription{}, err
	}

	if payment.Status != model.PaymentStatusPending {
		return model.Payment{}, model.UserSubscription{}, errs.ErrPaymentNotPending
	}

	now := time.Now().UTC()
	endDate := params.StartDate.AddDate(0, payment.DurationMonthsSnapshot, 0)
	userSub := model.UserSubscription{
		UserSubscriptionID:     newUUID(),
		UserID:                 payment.UserID,
		SubscriptionID:         payment.SubscriptionID,
		SourcePaymentID:        payment.PaymentID,
		PackageNameSnapshot:    payment.PackageNameSnapshot,
		DurationMonthsSnapshot: payment.DurationMonthsSnapshot,
		PriceAmountSnapshot:    payment.PriceAmountSnapshot,
		StartDate:              params.StartDate,
		EndDate:                endDate,
		CreatedAt:              now,
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_subscriptions (
			user_subscription_id, user_id, subscription_id, source_payment_id, package_name_snapshot,
			duration_months_snapshot, price_amount_snapshot, start_date, end_date, created_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`,
		userSub.UserSubscriptionID,
		userSub.UserID,
		userSub.SubscriptionID,
		userSub.SourcePaymentID,
		userSub.PackageNameSnapshot,
		userSub.DurationMonthsSnapshot,
		userSub.PriceAmountSnapshot,
		userSub.StartDate,
		userSub.EndDate,
		userSub.CreatedAt,
	)
	if err != nil {
		return model.Payment{}, model.UserSubscription{}, fmt.Errorf("insert user subscription: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE payments
		SET status = $2,
			admin_note = $3,
			proof_document_id = $4,
			verified_by = $5,
			verified_at = $6,
			paid_at = $7,
			updated_at = $8
		WHERE payment_id = $1
	`,
		payment.PaymentID,
		model.PaymentStatusSuccess,
		nullTrimmedString(params.AdminNote),
		nullTrimmedString(params.ProofDocumentID),
		nullTrimmedString(&params.VerifiedBy),
		now,
		now,
		now,
	)
	if err != nil {
		return model.Payment{}, model.UserSubscription{}, fmt.Errorf("update payment success status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.Payment{}, model.UserSubscription{}, fmt.Errorf("commit payment success: %w", err)
	}

	payment.Status = model.PaymentStatusSuccess
	payment.AdminNote = params.AdminNote
	payment.ProofDocumentID = params.ProofDocumentID
	payment.VerifiedBy = &params.VerifiedBy
	payment.VerifiedAt = &now
	payment.PaidAt = &now
	payment.UpdatedAt = now

	return payment, userSub, nil
}

func (r *DBPaymentRepository) MarkPaymentFailed(ctx context.Context, params MarkPaymentFailedParams) (model.Payment, error) {
	query := `
		UPDATE payments
		SET status = $2,
			admin_note = $3,
			proof_document_id = $4,
			verified_by = $5,
			verified_at = $6,
			updated_at = $7
		WHERE payment_id = $1
		  AND status = $8
		RETURNING payment_id, transaction_id, user_id, subscription_id, package_name_snapshot, duration_months_snapshot,
		          price_amount_snapshot, normal_price_snapshot, discount_price_snapshot, benefits_snapshot_json, payment_channel, qris_image_url,
		          status, admin_note, proof_document_id, verified_by, verified_at, paid_at, expired_at, created_at, updated_at
	`
	now := time.Now().UTC()

	payment, err := r.scanPayment(r.db.QueryRowContext(
		ctx,
		query,
		params.PaymentID,
		model.PaymentStatusFailed,
		nullTrimmedString(params.AdminNote),
		nullTrimmedString(params.ProofDocumentID),
		nullTrimmedString(&params.VerifiedBy),
		now,
		now,
		model.PaymentStatusPending,
	))
	if err == nil {
		return payment, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return model.Payment{}, err
	}

	existingStatus := model.PaymentStatus("")
	checkErr := r.db.QueryRowContext(ctx, `SELECT status FROM payments WHERE payment_id = $1`, params.PaymentID).Scan(&existingStatus)
	if checkErr != nil {
		if errors.Is(checkErr, sql.ErrNoRows) {
			return model.Payment{}, errs.ErrPaymentNotFound
		}
		return model.Payment{}, fmt.Errorf("check payment status before fail update: %w", checkErr)
	}

	return model.Payment{}, errs.ErrPaymentNotPending
}

func (r *DBPaymentRepository) scanPayment(row scanner) (model.Payment, error) {
	var payment model.Payment
	var benefitsJSON []byte
	var adminNote sql.NullString
	var proofDocumentID sql.NullString
	var verifiedBy sql.NullString
	var verifiedAt sql.NullTime
	var paidAt sql.NullTime
	var normalPriceSnapshot sql.NullInt64
	var discountPriceSnapshot sql.NullInt64

	err := row.Scan(
		&payment.PaymentID,
		&payment.TransactionID,
		&payment.UserID,
		&payment.SubscriptionID,
		&payment.PackageNameSnapshot,
		&payment.DurationMonthsSnapshot,
		&payment.PriceAmountSnapshot,
		&normalPriceSnapshot,
		&discountPriceSnapshot,
		&benefitsJSON,
		&payment.PaymentChannel,
		&payment.QrisImageURL,
		&payment.Status,
		&adminNote,
		&proofDocumentID,
		&verifiedBy,
		&verifiedAt,
		&paidAt,
		&payment.ExpiredAt,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	)
	if err != nil {
		return model.Payment{}, err
	}

	if len(benefitsJSON) > 0 {
		if err := json.Unmarshal(benefitsJSON, &payment.BenefitsSnapshot); err != nil {
			return model.Payment{}, fmt.Errorf("unmarshal payment benefits: %w", err)
		}
	}
	if adminNote.Valid {
		payment.AdminNote = &adminNote.String
	}
	if proofDocumentID.Valid {
		payment.ProofDocumentID = &proofDocumentID.String
	}
	if verifiedBy.Valid {
		payment.VerifiedBy = &verifiedBy.String
	}
	if verifiedAt.Valid {
		payment.VerifiedAt = &verifiedAt.Time
	}
	if paidAt.Valid {
		payment.PaidAt = &paidAt.Time
	}
	if normalPriceSnapshot.Valid {
		payment.NormalPriceSnapshot = &normalPriceSnapshot.Int64
	}
	if discountPriceSnapshot.Valid {
		payment.DiscountPriceSnapshot = &discountPriceSnapshot.Int64
	}

	return payment, nil
}

func scanSubscription(row scanner) (model.Subscription, error) {
	var sub model.Subscription
	var benefitsJSON []byte
	err := row.Scan(
		&sub.SubscriptionID,
		&sub.PackageKey,
		&sub.Name,
		&sub.Description,
		&sub.DurationMonths,
		&sub.PriceAmount,
		&benefitsJSON,
		&sub.IsActive,
		&sub.CreatedAt,
		&sub.UpdatedAt,
		&sub.NormalPriceAmount,
		&sub.DiscountPriceAmount,
	)
	if err != nil {
		return model.Subscription{}, err
	}

	if len(benefitsJSON) > 0 {
		if err := json.Unmarshal(benefitsJSON, &sub.Benefits); err != nil {
			return model.Subscription{}, fmt.Errorf("unmarshal subscription benefits: %w", err)
		}
	}

	return sub, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func nullTrimmedString(value *string) any {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return strings.TrimSpace(*value)
}

func newUUID() string {
	return uuid.NewString()
}
