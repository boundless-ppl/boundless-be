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
func (c *fakeConn) Begin() (driver.Tx, error) { return nil, errors.New("not implemented") }
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
			"price_amount_snapshot", "benefits_snapshot_json", "payment_channel", "qris_image_url",
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
