package repository_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"
)

type fakeResult struct{ affected int64 }

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) { return r.affected, nil }

type fakeRows struct {
	columns []string
	rows    [][]driver.Value
	idx     int
}

func (r *fakeRows) Columns() []string { return r.columns }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.idx])
	r.idx++
	return nil
}

type fakeBehavior struct {
	execFn  func(query string, args []driver.NamedValue) (driver.Result, error)
	queryFn func(query string, args []driver.NamedValue) (driver.Rows, error)
}

type fakeDriver struct{ behavior *fakeBehavior }

func (d *fakeDriver) Open(name string) (driver.Conn, error) {
	return &fakeConn{behavior: d.behavior}, nil
}

type fakeConn struct{ behavior *fakeBehavior }

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (c *fakeConn) Close() error              { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) { return &fakeTx{}, nil }
func (c *fakeConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.behavior.execFn == nil {
		return fakeResult{affected: 1}, nil
	}
	return c.behavior.execFn(query, args)
}
func (c *fakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.behavior.queryFn == nil {
		return &fakeRows{columns: []string{"c"}, rows: [][]driver.Value{}}, nil
	}
	return c.behavior.queryFn(query, args)
}

type fakeTx struct{}

func (t *fakeTx) Commit() error   { return nil }
func (t *fakeTx) Rollback() error { return nil }

var seq int

func newFakeDB(t *testing.T, behavior *fakeBehavior) *sql.DB {
	t.Helper()
	seq++
	driverName := fmt.Sprintf("payment_repo_fakedb_%d", seq)
	sql.Register(driverName, &fakeDriver{behavior: behavior})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open fake db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestPaymentRepositoryImplementsContract(t *testing.T) {
	var _ repository.PaymentRepository = (*repository.DBPaymentRepository)(nil)
}

func TestCreatePaymentRepository(t *testing.T) {
	db := newFakeDB(t, &fakeBehavior{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		if !strings.Contains(query, "INSERT INTO payments") {
			t.Fatalf("unexpected query: %s", query)
		}
		return fakeResult{affected: 1}, nil
	}})
	repo := repository.NewPaymentRepository(db)
	now := time.Now().UTC()
	exp := now.Add(time.Hour)

	_, err := repo.CreatePayment(context.Background(), model.Payment{
		PaymentID:              "pay-1",
		TransactionID:          "tx-1",
		UserID:                 "user-1",
		SubscriptionID:         "sub-1",
		PackageNameSnapshot:    "The Scholar",
		DurationMonthsSnapshot: 3,
		PriceAmountSnapshot:    100,
		BenefitsSnapshot:       []string{"a"},
		PaymentChannel:         "qris_manual",
		QrisImageURL:           "-",
		Status:                 model.PaymentStatusPending,
		ExpiredAt:              &exp,
		CreatedAt:              now,
		UpdatedAt:              now,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestFindPaymentByIDAndUserNotFoundRepository(t *testing.T) {
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{columns: []string{
			"payment_id", "transaction_id", "user_id", "subscription_id", "package_name_snapshot", "duration_months_snapshot",
			"price_amount_snapshot", "normal_price_snapshot", "discount_price_snapshot", "benefits_snapshot_json", "payment_channel", "qris_image_url",
			"status", "admin_note", "proof_document_id", "verified_by", "verified_at", "paid_at", "expired_at", "created_at", "updated_at",
		}, rows: [][]driver.Value{}}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	_, err := repo.FindPaymentByIDAndUser(context.Background(), "missing", "user-1")
	if !errors.Is(err, errs.ErrPaymentNotFound) {
		t.Fatalf("expected %v, got %v", errs.ErrPaymentNotFound, err)
	}
}

func TestAttachPaymentProofDocumentNotPendingRepository(t *testing.T) {
	db := newFakeDB(t, &fakeBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			if strings.Contains(query, "UPDATE payments") {
				return fakeResult{affected: 0}, nil
			}
			return fakeResult{affected: 0}, nil
		},
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if strings.Contains(query, "SELECT status FROM payments") {
				return &fakeRows{columns: []string{"status"}, rows: [][]driver.Value{{"success"}}}, nil
			}
			return &fakeRows{columns: []string{"c"}, rows: [][]driver.Value{}}, nil
		},
	})
	repo := repository.NewPaymentRepository(db)

	err := repo.AttachPaymentProofDocument(context.Background(), "pay-1", "user-1", "doc-1")
	if !errors.Is(err, errs.ErrPaymentNotPending) {
		t.Fatalf("expected %v, got %v", errs.ErrPaymentNotPending, err)
	}
}

func TestListActiveSubscriptionsRepository(t *testing.T) {
	now := time.Now().UTC()
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		if !strings.Contains(query, "FROM subscriptions") {
			t.Fatalf("unexpected query: %s", query)
		}
		return &fakeRows{columns: []string{"subscription_id", "package_key", "name", "description", "duration_months", "price_amount", "benefits_json", "is_active", "created_at", "updated_at", "normal_price_amount", "discount_price_amount"}, rows: [][]driver.Value{{"sub-1", "scholar", "The Scholar", "desc", int64(3), int64(100), []byte(`["a","b"]`), true, now, now, int64(149000), int64(39000)}}}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	items, err := repo.ListActiveSubscriptions(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(items) != 1 || items[0].SubscriptionID != "sub-1" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestFindActiveSubscriptionByIDNotFoundRepository(t *testing.T) {
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{columns: []string{"subscription_id", "package_key", "name", "description", "duration_months", "price_amount", "benefits_json", "is_active", "created_at", "updated_at", "normal_price_amount", "discount_price_amount"}, rows: [][]driver.Value{}}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	_, err := repo.FindActiveSubscriptionByID(context.Background(), "missing")
	if !errors.Is(err, errs.ErrSubscriptionNotFound) {
		t.Fatalf("expected %v, got %v", errs.ErrSubscriptionNotFound, err)
	}
}

func TestCreateDocumentRepository(t *testing.T) {
	db := newFakeDB(t, &fakeBehavior{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		if !strings.Contains(query, "INSERT INTO documents") {
			t.Fatalf("unexpected query: %s", query)
		}
		return fakeResult{affected: 1}, nil
	}})
	repo := repository.NewPaymentRepository(db)
	now := time.Now().UTC()

	_, err := repo.CreateDocument(context.Background(), model.Document{
		DocumentID:       "doc-1",
		UserID:           "user-1",
		OriginalFilename: "proof.pdf",
		StoragePath:      "uploads/proof.pdf",
		PublicURL:        "http://local/proof.pdf",
		MIMEType:         "application/pdf",
		SizeBytes:        123,
		DocumentType:     model.DocumentTypePaymentProof,
		UploadedAt:       now,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestFindPaymentByIDRepository(t *testing.T) {
	now := time.Now().UTC()
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{columns: []string{"payment_id", "transaction_id", "user_id", "subscription_id", "package_name_snapshot", "duration_months_snapshot", "price_amount_snapshot", "normal_price_snapshot", "discount_price_snapshot", "benefits_snapshot_json", "payment_channel", "qris_image_url", "status", "admin_note", "proof_document_id", "verified_by", "verified_at", "paid_at", "expired_at", "created_at", "updated_at"}, rows: [][]driver.Value{{"pay-1", "tx-1", "user-1", "sub-1", "The Scholar", int64(3), int64(100), int64(149000), int64(39000), []byte(`["a"]`), "qris_manual", "-", "pending", nil, nil, nil, nil, nil, now.Add(time.Hour), now, now}}}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	out, err := repo.FindPaymentByID(context.Background(), "pay-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.PaymentID != "pay-1" {
		t.Fatalf("unexpected payment: %+v", out)
	}
}

func TestFindPremiumCoverageEndAtRepository(t *testing.T) {
	end := time.Now().UTC().AddDate(0, 1, 0)
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{columns: []string{"max"}, rows: [][]driver.Value{{end}}}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	out, err := repo.FindPremiumCoverageEndAt(context.Background(), "user-1", time.Now().UTC())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out == nil {
		t.Fatal("expected non-nil end date")
	}
}

func TestFindCurrentPremiumSubscriptionRepositoryMergesContiguousRows(t *testing.T) {
	start1 := time.Date(2026, time.March, 19, 13, 48, 14, 0, time.UTC)
	start2 := start1.AddDate(0, 3, 0)
	start3 := start2.AddDate(0, 3, 0)
	ref := start1.AddDate(0, 1, 0)
	end3 := start3.AddDate(0, 3, 0)
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		if !strings.Contains(query, "FROM user_subscriptions") {
			return &fakeRows{columns: []string{"c"}, rows: [][]driver.Value{}}, nil
		}
		return &fakeRows{columns: []string{"user_subscription_id", "user_id", "subscription_id", "source_payment_id", "package_name_snapshot", "duration_months_snapshot", "price_amount_snapshot", "start_date", "end_date", "created_at"}, rows: [][]driver.Value{
			{"us-1", "user-1", "sub-1", "pay-1", "The Scholar", int64(3), int64(100), start1, start2, start1},
			{"us-2", "user-1", "sub-1", "pay-2", "The Scholar", int64(3), int64(100), start2, start3, start2},
			{"us-3", "user-1", "sub-1", "pay-3", "The Scholar", int64(3), int64(100), start3, end3, start3},
		}}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	out, err := repo.FindCurrentPremiumSubscription(context.Background(), "user-1", ref)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !out.StartDate.Equal(start1) {
		t.Fatalf("expected start %s, got %s", start1, out.StartDate)
	}
	if !out.EndDate.Equal(end3) {
		t.Fatalf("expected end %s, got %s", end3, out.EndDate)
	}
}

func TestListAdminPaymentsRepository(t *testing.T) {
	now := time.Now().UTC()
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		proofID := "doc-1"
		proofURL := "http://local/doc-1"
		expiredAt := now.Add(24 * time.Hour)
		return &fakeRows{columns: []string{"payment_id", "transaction_id", "user_id", "nama_lengkap", "email", "package_name_snapshot", "price_amount_snapshot", "normal_amount", "status", "created_at", "expired_at", "proof_document_id", "public_url"}, rows: [][]driver.Value{{"pay-1", "tx-1", "user-1", "Alice", "alice@example.com", "The Scholar", int64(100), int64(1000), "pending", now, expiredAt, proofID, proofURL}}}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	items, err := repo.ListAdminPayments(context.Background(), repository.PaymentListParams{Since: now.Add(-time.Hour), Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(items) != 1 || items[0].PaymentID != "pay-1" {
		t.Fatalf("unexpected items: %+v", items)
	}
	if items[0].UserEmail != "alice@example.com" {
		t.Fatalf("expected user email alice@example.com, got %s", items[0].UserEmail)
	}
	if items[0].ExpiredAt.IsZero() {
		t.Fatalf("expected expired_at to be populated")
	}
}

func TestListPendingPaymentNotificationsRepository(t *testing.T) {
	now := time.Now().UTC()
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{columns: []string{"payment_id", "transaction_id", "user_id", "nama_lengkap", "email", "package_name_snapshot", "price_amount_snapshot", "public_url", "created_at"}, rows: [][]driver.Value{{"pay-1", "tx-1", "user-1", "Alice", "alice@example.com", "The Scholar", int64(100), "http://local/doc-1", now}}}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	items, err := repo.ListPendingPaymentNotifications(context.Background(), 5)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(items) != 1 || items[0].PaymentID != "pay-1" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestMarkPaymentNotificationSentRepository(t *testing.T) {
	db := newFakeDB(t, &fakeBehavior{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		if !strings.Contains(query, "admin_notified_at") {
			t.Fatalf("unexpected query: %s", query)
		}
		return fakeResult{affected: 1}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	err := repo.MarkPaymentNotificationSent(context.Background(), "pay-1", time.Now().UTC())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestMarkPaymentSuccessBeginTxErrorRepository(t *testing.T) {
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return nil, errors.New("query failed")
	}})
	repo := repository.NewPaymentRepository(db)

	_, _, err := repo.MarkPaymentSuccess(context.Background(), repository.MarkPaymentSuccessParams{PaymentID: "pay-1", VerifiedBy: "admin-1", StartDate: time.Now().UTC()})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFindUserSubscriptionByPaymentIDRepository(t *testing.T) {
	now := time.Now().UTC()
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		if strings.Contains(query, "FROM user_subscriptions") {
			return &fakeRows{columns: []string{"user_subscription_id", "user_id", "subscription_id", "source_payment_id", "package_name_snapshot", "duration_months_snapshot", "price_amount_snapshot", "start_date", "end_date", "created_at"}, rows: [][]driver.Value{{"us-1", "user-1", "sub-1", "pay-1", "The Scholar", int64(3), int64(100), now, now.AddDate(0, 3, 0), now}}}, nil
		}
		return &fakeRows{columns: []string{"c"}, rows: [][]driver.Value{}}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	out, err := repo.FindUserSubscriptionByPaymentID(context.Background(), "pay-1", "user-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.UserSubscriptionID != "us-1" {
		t.Fatalf("unexpected subscription: %+v", out)
	}
}

func TestMarkPaymentSuccessRepository(t *testing.T) {
	now := time.Now().UTC()
	db := newFakeDB(t, &fakeBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if strings.Contains(query, "FOR UPDATE") {
				return &fakeRows{columns: []string{"payment_id", "transaction_id", "user_id", "subscription_id", "package_name_snapshot", "duration_months_snapshot", "price_amount_snapshot", "normal_price_snapshot", "discount_price_snapshot", "benefits_snapshot_json", "payment_channel", "qris_image_url", "status", "admin_note", "proof_document_id", "verified_by", "verified_at", "paid_at", "expired_at", "created_at", "updated_at"}, rows: [][]driver.Value{{"pay-1", "tx-1", "user-1", "sub-1", "The Scholar", int64(3), int64(100), int64(149000), int64(39000), []byte(`["a"]`), "qris_manual", "-", "pending", nil, nil, nil, nil, nil, now.Add(time.Hour), now, now}}}, nil
			}
			return &fakeRows{columns: []string{"c"}, rows: [][]driver.Value{}}, nil
		},
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			return fakeResult{affected: 1}, nil
		},
	})
	repo := repository.NewPaymentRepository(db)

	payment, sub, err := repo.MarkPaymentSuccess(context.Background(), repository.MarkPaymentSuccessParams{
		PaymentID:  "pay-1",
		VerifiedBy: "admin-1",
		StartDate:  now,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if payment.Status != model.PaymentStatusSuccess || sub.UserSubscriptionID == "" {
		t.Fatalf("unexpected outputs payment=%+v sub=%+v", payment, sub)
	}
}

func TestMarkPaymentFailedNotFoundRepository(t *testing.T) {
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		if strings.Contains(query, "RETURNING") {
			return &fakeRows{columns: []string{"payment_id", "transaction_id", "user_id", "subscription_id", "package_name_snapshot", "duration_months_snapshot", "price_amount_snapshot", "normal_price_snapshot", "discount_price_snapshot", "benefits_snapshot_json", "payment_channel", "qris_image_url", "status", "admin_note", "proof_document_id", "verified_by", "verified_at", "paid_at", "expired_at", "created_at", "updated_at"}, rows: [][]driver.Value{}}, nil
		}
		if strings.Contains(query, "SELECT status FROM payments") {
			return &fakeRows{columns: []string{"status"}, rows: [][]driver.Value{}}, nil
		}
		return &fakeRows{columns: []string{"c"}, rows: [][]driver.Value{}}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	_, err := repo.MarkPaymentFailed(context.Background(), repository.MarkPaymentFailedParams{PaymentID: "missing", VerifiedBy: "admin-1"})
	if !errors.Is(err, errs.ErrPaymentNotFound) {
		t.Fatalf("expected %v, got %v", errs.ErrPaymentNotFound, err)
	}
}

func TestFindActiveSubscriptionByIDRepository(t *testing.T) {
	now := time.Now().UTC()
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{columns: []string{"subscription_id", "package_key", "name", "description", "duration_months", "price_amount", "benefits_json", "is_active", "created_at", "updated_at", "normal_price_amount", "discount_price_amount"}, rows: [][]driver.Value{{"sub-1", "scholar", "The Scholar", "desc", int64(3), int64(100), []byte(`["a"]`), true, now, now, int64(149000), int64(39000)}}}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	out, err := repo.FindActiveSubscriptionByID(context.Background(), "sub-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.SubscriptionID != "sub-1" {
		t.Fatalf("unexpected subscription: %+v", out)
	}
}

func TestFindPaymentByIDNotFoundRepository(t *testing.T) {
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{columns: []string{"payment_id", "transaction_id", "user_id", "subscription_id", "package_name_snapshot", "duration_months_snapshot", "price_amount_snapshot", "normal_price_snapshot", "discount_price_snapshot", "benefits_snapshot_json", "payment_channel", "qris_image_url", "status", "admin_note", "proof_document_id", "verified_by", "verified_at", "paid_at", "expired_at", "created_at", "updated_at"}, rows: [][]driver.Value{}}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	_, err := repo.FindPaymentByID(context.Background(), "missing")
	if !errors.Is(err, errs.ErrPaymentNotFound) {
		t.Fatalf("expected %v, got %v", errs.ErrPaymentNotFound, err)
	}
}

func TestAttachPaymentProofDocumentSuccessRepository(t *testing.T) {
	db := newFakeDB(t, &fakeBehavior{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return fakeResult{affected: 1}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	err := repo.AttachPaymentProofDocument(context.Background(), "pay-1", "user-1", "doc-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestAttachPaymentProofDocumentNotFoundRepository(t *testing.T) {
	db := newFakeDB(t, &fakeBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			if strings.Contains(query, "UPDATE payments") {
				return fakeResult{affected: 0}, nil
			}
			return fakeResult{affected: 0}, nil
		},
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if strings.Contains(query, "SELECT status FROM payments") {
				return &fakeRows{columns: []string{"status"}, rows: [][]driver.Value{}}, nil
			}
			return &fakeRows{columns: []string{"c"}, rows: [][]driver.Value{}}, nil
		},
	})
	repo := repository.NewPaymentRepository(db)

	err := repo.AttachPaymentProofDocument(context.Background(), "pay-1", "user-1", "doc-1")
	if !errors.Is(err, errs.ErrPaymentNotFound) {
		t.Fatalf("expected %v, got %v", errs.ErrPaymentNotFound, err)
	}
}

func TestMarkPaymentSuccessNotPendingRepository(t *testing.T) {
	now := time.Now().UTC()
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		if strings.Contains(query, "FOR UPDATE") {
			return &fakeRows{columns: []string{"payment_id", "transaction_id", "user_id", "subscription_id", "package_name_snapshot", "duration_months_snapshot", "price_amount_snapshot", "normal_price_snapshot", "discount_price_snapshot", "benefits_snapshot_json", "payment_channel", "qris_image_url", "status", "admin_note", "proof_document_id", "verified_by", "verified_at", "paid_at", "expired_at", "created_at", "updated_at"}, rows: [][]driver.Value{{"pay-1", "tx-1", "user-1", "sub-1", "The Scholar", int64(3), int64(100), int64(149000), int64(39000), []byte(`["a"]`), "qris_manual", "-", "success", nil, nil, nil, nil, nil, now.Add(time.Hour), now, now}}}, nil
		}
		return &fakeRows{columns: []string{"c"}, rows: [][]driver.Value{}}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	_, _, err := repo.MarkPaymentSuccess(context.Background(), repository.MarkPaymentSuccessParams{PaymentID: "pay-1", VerifiedBy: "admin-1", StartDate: now})
	if !errors.Is(err, errs.ErrPaymentNotPending) {
		t.Fatalf("expected %v, got %v", errs.ErrPaymentNotPending, err)
	}
}

func TestMarkPaymentFailedNotPendingRepository(t *testing.T) {
	db := newFakeDB(t, &fakeBehavior{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		if strings.Contains(query, "RETURNING") {
			return &fakeRows{columns: []string{"payment_id", "transaction_id", "user_id", "subscription_id", "package_name_snapshot", "duration_months_snapshot", "price_amount_snapshot", "normal_price_snapshot", "discount_price_snapshot", "benefits_snapshot_json", "payment_channel", "qris_image_url", "status", "admin_note", "proof_document_id", "verified_by", "verified_at", "paid_at", "expired_at", "created_at", "updated_at"}, rows: [][]driver.Value{}}, nil
		}
		if strings.Contains(query, "SELECT status FROM payments") {
			return &fakeRows{columns: []string{"status"}, rows: [][]driver.Value{{"success"}}}, nil
		}
		return &fakeRows{columns: []string{"c"}, rows: [][]driver.Value{}}, nil
	}})
	repo := repository.NewPaymentRepository(db)

	_, err := repo.MarkPaymentFailed(context.Background(), repository.MarkPaymentFailedParams{PaymentID: "pay-1", VerifiedBy: "admin-1"})
	if !errors.Is(err, errs.ErrPaymentNotPending) {
		t.Fatalf("expected %v, got %v", errs.ErrPaymentNotPending, err)
	}
}
