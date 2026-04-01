package database_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"testing"

	"boundless-be/database"
)

func TestNewConnectionRequiresDatabaseURL(t *testing.T) {
	db, err := database.NewConnection("")
	if err != database.ErrMissingDatabaseURL {
		t.Fatalf("expected %v, got %v", database.ErrMissingDatabaseURL, err)
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

	db, err := database.NewConnectionWithOpen("fake-dsn", func(driverNameArg, dsn string) (*sql.DB, error) {
		return sql.Open(driverName, dsn)
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if db == nil {
		t.Fatal("expected db")
	}
	_ = db.Close()
}
