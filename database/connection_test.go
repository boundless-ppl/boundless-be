package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestNewConnectionRequiresDatabaseURL(t *testing.T) {
	db, err := NewConnection("")
	if err != ErrMissingDatabaseURL {
		t.Fatalf("expected %v, got %v", ErrMissingDatabaseURL, err)
	}
	if db != nil {
		t.Fatal("expected nil db")
	}
}

type fakePingDriver struct{}

func (d *fakePingDriver) Open(name string) (driver.Conn, error) {
	return &fakePingConn{}, nil
}

type fakePingConn struct{}

func (c *fakePingConn) Prepare(query string) (driver.Stmt, error) { return nil, io.EOF }
func (c *fakePingConn) Close() error                              { return nil }
func (c *fakePingConn) Begin() (driver.Tx, error)                 { return nil, io.EOF }
func (c *fakePingConn) Ping(ctx context.Context) error            { return nil }

var fakePingSeq int

func TestNewConnectionSuccessfulPing(t *testing.T) {
	fakePingSeq++
	driverName := fmt.Sprintf("fakeping_%d", fakePingSeq)
	sql.Register(driverName, &fakePingDriver{})

	origOpenDB := databaseOpenDB
	databaseOpenDB = func(_ pgx.ConnConfig) *sql.DB {
		db, err := sql.Open(driverName, "")
		if err != nil {
			t.Fatalf("expected nil sql open error, got %v", err)
		}
		return db
	}
	defer func() { databaseOpenDB = origOpenDB }()

	db, err := NewConnection("postgres://user:pass@localhost:5432/db")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if db == nil {
		t.Fatal("expected db")
	}
	_ = db.Close()
}
