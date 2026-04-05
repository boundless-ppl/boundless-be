package repository_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"testing"

	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"
)

type fakeUnivResult struct {
	affected int64
	err      error
}

func (r fakeUnivResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeUnivResult) RowsAffected() (int64, error) { return r.affected, r.err }

type fakeUnivRows struct {
	columns []string
	rows    [][]driver.Value
	idx     int
}

func (r *fakeUnivRows) Columns() []string { return r.columns }
func (r *fakeUnivRows) Close() error      { return nil }
func (r *fakeUnivRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.idx])
	r.idx++
	return nil
}

type fakeUnivDBBehavior struct {
	execFn  func(query string, args []driver.NamedValue) (driver.Result, error)
	queryFn func(query string, args []driver.NamedValue) (driver.Rows, error)
}

type fakeUnivDriver struct {
	behavior *fakeUnivDBBehavior
}

func (d *fakeUnivDriver) Open(name string) (driver.Conn, error) {
	return &fakeUnivConn{behavior: d.behavior}, nil
}

type fakeUnivConn struct {
	behavior *fakeUnivDBBehavior
}

func (c *fakeUnivConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (c *fakeUnivConn) Close() error              { return nil }
func (c *fakeUnivConn) Begin() (driver.Tx, error) { return nil, errors.New("not implemented") }
func (c *fakeUnivConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.behavior.execFn == nil {
		return fakeUnivResult{affected: 1}, nil
	}
	return c.behavior.execFn(query, args)
}
func (c *fakeUnivConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.behavior.queryFn == nil {
		return &fakeUnivRows{columns: []string{"col"}, rows: [][]driver.Value{}}, nil
	}
	return c.behavior.queryFn(query, args)
}

var fakeUnivDriverSeq int

func newFakeUnivDB(t *testing.T, behavior *fakeUnivDBBehavior) *sql.DB {
	t.Helper()
	fakeUnivDriverSeq++
	driverName := fmt.Sprintf("fakedb_univ_%d", fakeUnivDriverSeq)
	sql.Register(driverName, &fakeUnivDriver{behavior: behavior})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open fake university db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestUniversityRepositoryImplementsContract(t *testing.T) {
	var _ repository.UniversityRepository = (*repository.DBUniversityRepository)(nil)
}

func TestUniversityRepositoryConstructor(t *testing.T) {
	if repository.NewUniversityRepository(nil) == nil {
		t.Fatal("expected non nil repository")
	}
}

func TestCreateUniversityRepository(t *testing.T) {
	db := newFakeUnivDB(t, &fakeUnivDBBehavior{})
	repo := repository.NewUniversityRepository(db)

	u, err := repo.Create(context.Background(), model.University{
		ID:       "u1",
		NegaraID: "JP",
		Nama:     "University A",
		Kota:     "Tokyo",
		Tipe:     model.NATIONAL,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if u.ID != "u1" {
		t.Fatalf("expected u1, got %s", u.ID)
	}
}

func TestFindAllUniversitiesRepository(t *testing.T) {
	db := newFakeUnivDB(t, &fakeUnivDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			return &fakeUnivRows{
				columns: []string{"id", "negara_id", "nama", "kota", "tipe", "deskripsi", "website", "ranking"},
				rows: [][]driver.Value{
					{"u1", "JP", "University A", "Tokyo", string(model.NATIONAL), "desc", "site", int64(1)},
					{"u2", "SG", "University B", "Singapore", string(model.PRIVATE), "desc2", "site2", nil},
				},
			}, nil
		},
	})
	repo := repository.NewUniversityRepository(db)

	result, err := repo.FindAll(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result))
	}
}

func TestFindUniversityByIDRepository(t *testing.T) {
	db := newFakeUnivDB(t, &fakeUnivDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			return &fakeUnivRows{
				columns: []string{"id", "negara_id", "nama", "kota", "tipe", "deskripsi", "website", "ranking"},
				rows: [][]driver.Value{
					{"u1", "JP", "University A", "Tokyo", string(model.NATIONAL), "desc", "site", int64(1)},
				},
			}, nil
		},
	})
	repo := repository.NewUniversityRepository(db)

	result, err := repo.FindByID(context.Background(), "u1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.ID != "u1" {
		t.Fatalf("expected u1, got %s", result.ID)
	}
}

func TestFindUniversityByIDNotFoundRepository(t *testing.T) {
	db := newFakeUnivDB(t, &fakeUnivDBBehavior{
		queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			return &fakeUnivRows{
				columns: []string{"id", "negara_id", "nama", "kota", "tipe", "deskripsi", "website", "ranking"},
				rows:    [][]driver.Value{},
			}, nil
		},
	})
	repo := repository.NewUniversityRepository(db)

	_, err := repo.FindByID(context.Background(), "missing")
	if !errors.Is(err, errs.ErrUniversityNotFound) {
		t.Fatalf("expected %v, got %v", errs.ErrUniversityNotFound, err)
	}
}

func TestUpdateUniversityRepository(t *testing.T) {
	db := newFakeUnivDB(t, &fakeUnivDBBehavior{})
	repo := repository.NewUniversityRepository(db)

	u, err := repo.Update(context.Background(), model.University{
		ID:       "u1",
		NegaraID: "JP",
		Nama:     "Updated",
		Kota:     "Tokyo",
		Tipe:     model.NATIONAL,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if u.Nama != "Updated" {
		t.Fatalf("expected Updated, got %s", u.Nama)
	}
}

func TestUpdateUniversityNotFoundRepository(t *testing.T) {
	db := newFakeUnivDB(t, &fakeUnivDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			return fakeUnivResult{affected: 0}, nil
		},
	})
	repo := repository.NewUniversityRepository(db)

	_, err := repo.Update(context.Background(), model.University{ID: "missing"})
	if !errors.Is(err, errs.ErrUniversityNotFound) {
		t.Fatalf("expected %v, got %v", errs.ErrUniversityNotFound, err)
	}
}

func TestDeleteUniversityRepository(t *testing.T) {
	db := newFakeUnivDB(t, &fakeUnivDBBehavior{})
	repo := repository.NewUniversityRepository(db)

	if err := repo.Delete(context.Background(), "u1"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestDeleteUniversityNotFoundRepository(t *testing.T) {
	db := newFakeUnivDB(t, &fakeUnivDBBehavior{
		execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
			return fakeUnivResult{affected: 0}, nil
		},
	})
	repo := repository.NewUniversityRepository(db)

	err := repo.Delete(context.Background(), "missing")
	if !errors.Is(err, errs.ErrUniversityNotFound) {
		t.Fatalf("expected %v, got %v", errs.ErrUniversityNotFound, err)
	}
}
