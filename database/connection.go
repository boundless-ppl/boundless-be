package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	// Register the pgx driver with database/sql so sql.Open("pgx", ...) works.
	_ "github.com/jackc/pgx/v5/stdlib"
)

var ErrMissingDatabaseURL = errors.New("DATABASE_URL is required")

var databaseOpen = sql.Open

func NewConnection(databaseURL string) (*sql.DB, error) {
	if databaseURL == "" {
		return nil, ErrMissingDatabaseURL
	}

	db, err := databaseOpen("pgx", databaseURL)
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
