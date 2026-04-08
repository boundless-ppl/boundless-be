package database

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"time"

	// Register the pgx driver with database/sql so sql.Open("pgx", ...) works.
	_ "github.com/jackc/pgx/v5/stdlib"
)

var ErrMissingDatabaseURL = errors.New("DATABASE_URL is required")

func NewConnection(databaseURL string) (*sql.DB, error) {
	return NewConnectionWithOpen(databaseURL, sql.Open)
}

func NewConnectionWithOpen(
	databaseURL string,
	open func(driverName, dsn string) (*sql.DB, error),
) (*sql.DB, error) {
	if databaseURL == "" {
		return nil, ErrMissingDatabaseURL
	}

	databaseURL = withSimpleProtocol(databaseURL)

	db, err := open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func withSimpleProtocol(databaseURL string) string {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return databaseURL
	}

	query := parsed.Query()
	if query.Get("default_query_exec_mode") == "" {
		query.Set("default_query_exec_mode", "simple_protocol")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
