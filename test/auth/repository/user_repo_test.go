package repository_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"boundless-be/model"
	"boundless-be/repository"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestRepositoryImplementsContractRepo(t *testing.T) {
	var _ repository.UserRepository = (*repository.DBUserRepository)(nil)
}

func TestRepositoryConstructorRepo(t *testing.T) {
	repo := repository.NewUserRepository(nil)
	if repo == nil {
		t.Fatal("expected non nil repository")
	}
}

type fakeResult struct {
	affected int64
	err      error
}

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) { return r.affected, r.err }

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

type fakeDBBehavior struct {
	execFn  func(query string, args []driver.NamedValue) (driver.Result, error)
	queryFn func(query string, args []driver.NamedValue) (driver.Rows, error)
}

type fakeDriver struct {
	behavior *fakeDBBehavior
}

func (d *fakeDriver) Open(name string) (driver.Conn, error) {
	return &fakeConn{behavior: d.behavior}, nil
}

type fakeConn struct {
	behavior *fakeDBBehavior
}

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
		return &fakeRows{columns: []string{"col"}, rows: [][]driver.Value{}}, nil
	}
	return c.behavior.queryFn(query, args)
}

var fakeDriverSeq int

func newFakeDB(t *testing.T, behavior *fakeDBBehavior) *sql.DB {
	t.Helper()
	fakeDriverSeq++
	driverName := fmt.Sprintf("fakedb_repo_%d", fakeDriverSeq)
	sql.Register(driverName, &fakeDriver{behavior: behavior})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("failed to open fake db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestCreateRepository(t *testing.T) {
	db := newFakeDB(t, &fakeDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			if query == "" || len(args) == 0 {
				t.Fatal("expected insert query and args")
			}
			return fakeResult{affected: 1}, nil
		},
	})
	repo := repository.NewUserRepository(db)
	user, err := model.NewUser("u1", "Alice", "admin", " ALICE@EXAMPLE.COM ", "hashed")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	got, err := repo.Create(context.Background(), user)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.Email != "alice@example.com" {
		t.Fatalf("expected normalized email, got %s", got.Email)
	}
}

func TestCreateDuplicateEmailRepository(t *testing.T) {
	db := newFakeDB(t, &fakeDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			return nil, &pgconn.PgError{Code: "23505"}
		},
	})
	repo := repository.NewUserRepository(db)
	user, _ := model.NewUser("u2", "Bob", "admin", "bob@example.com", "hashed")

	_, err := repo.Create(context.Background(), user)
	if !errors.Is(err, repository.ErrEmailExists) {
		t.Fatalf("expected %v, got %v", repository.ErrEmailExists, err)
	}
}

func TestCreateUnexpectedErrorRepository(t *testing.T) {
	db := newFakeDB(t, &fakeDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			return nil, errors.New("db down")
		},
	})
	repo := repository.NewUserRepository(db)
	user, _ := model.NewUser("u3", "Carol", "admin", "carol@example.com", "hashed")

	_, err := repo.Create(context.Background(), user)
	if err == nil || err.Error() != "db down" {
		t.Fatalf("expected db down error, got %v", err)
	}
}

func TestFindByEmailRepository(t *testing.T) {
	now := time.Now()
	db := newFakeDB(t, &fakeDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			return &fakeRows{
				columns: []string{"user_id", "nama_lengkap", "role", "email", "password_hash", "created_at", "failed_login_count", "first_failed_at", "locked_until"},
				rows: [][]driver.Value{
					{"u1", "Alice", "admin", "alice@example.com", "hashed", now, int64(1), now, now.Add(time.Minute)},
				},
			}, nil
		},
	})
	repo := repository.NewUserRepository(db)

	user, err := repo.FindByEmail(context.Background(), " ALICE@EXAMPLE.COM ")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if user.UserID != "u1" || user.Email != "alice@example.com" {
		t.Fatalf("unexpected user: %+v", user)
	}
	if user.FirstFailedAt.IsZero() || user.LockedUntil.IsZero() {
		t.Fatal("expected nullable times mapped")
	}
}

func TestFindByIDNotFoundRepository(t *testing.T) {
	db := newFakeDB(t, &fakeDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			return &fakeRows{
				columns: []string{"user_id", "nama_lengkap", "role", "email", "password_hash", "created_at", "failed_login_count", "first_failed_at", "locked_until"},
				rows:    [][]driver.Value{},
			}, nil
		},
	})
	repo := repository.NewUserRepository(db)

	_, err := repo.FindByID(context.Background(), "missing")
	if !errors.Is(err, repository.ErrUserNotFound) {
		t.Fatalf("expected %v, got %v", repository.ErrUserNotFound, err)
	}
}

func TestFindByEmailScanErrorRepository(t *testing.T) {
	db := newFakeDB(t, &fakeDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			return &fakeRows{
				columns: []string{"user_id", "nama_lengkap", "role", "email", "password_hash", "created_at", "failed_login_count", "first_failed_at", "locked_until"},
				rows: [][]driver.Value{
					{"u1", "Alice", "admin", "alice@example.com", "hashed", "bad-time", int64(1), nil, nil},
				},
			}, nil
		},
	})
	repo := repository.NewUserRepository(db)

	_, err := repo.FindByEmail(context.Background(), "alice@example.com")
	if err == nil {
		t.Fatal("expected scan error")
	}
}

func TestUpdateRepository(t *testing.T) {
	db := newFakeDB(t, &fakeDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			return fakeResult{affected: 1}, nil
		},
	})
	repo := repository.NewUserRepository(db)
	user := model.User{
		UserID:           "u1",
		NamaLengkap:      "Alice",
		Role:             "admin",
		Email:            " ALICE@EXAMPLE.COM ",
		PasswordHash:     "hashed",
		FailedLoginCount: 0,
		FirstFailedAt:    time.Now(),
		LockedUntil:      time.Now().Add(time.Minute),
	}

	if err := repo.Update(context.Background(), user); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUpdateExecErrorRepository(t *testing.T) {
	db := newFakeDB(t, &fakeDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			return nil, errors.New("exec failed")
		},
	})
	repo := repository.NewUserRepository(db)
	user := model.User{UserID: "u1", Email: "x@example.com"}

	err := repo.Update(context.Background(), user)
	if err == nil || err.Error() != "exec failed" {
		t.Fatalf("expected exec failed, got %v", err)
	}
}

func TestUpdateNotFoundRepository(t *testing.T) {
	db := newFakeDB(t, &fakeDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			return fakeResult{affected: 0}, nil
		},
	})
	repo := repository.NewUserRepository(db)
	user := model.User{UserID: "u-missing", Email: "x@example.com"}

	err := repo.Update(context.Background(), user)
	if !errors.Is(err, repository.ErrUserNotFound) {
		t.Fatalf("expected %v, got %v", repository.ErrUserNotFound, err)
	}
}

func TestUpdateRowsAffectedErrorRepository(t *testing.T) {
	db := newFakeDB(t, &fakeDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			return fakeResult{affected: 0, err: errors.New("rows affected error")}, nil
		},
	})
	repo := repository.NewUserRepository(db)
	user := model.User{UserID: "u1", Email: "x@example.com"}

	err := repo.Update(context.Background(), user)
	if err == nil || err.Error() != "rows affected error" {
		t.Fatalf("expected rows affected error, got %v", err)
	}
}
