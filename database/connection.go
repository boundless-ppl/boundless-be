package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

var ErrMissingDatabaseURL = errors.New("DATABASE_URL is required")

var databaseOpenDB = func(config pgx.ConnConfig) *sql.DB {
	return stdlib.OpenDB(config)
}

func NewConnection(databaseURL string) (*sql.DB, error) {
	if databaseURL == "" {
		return nil, ErrMissingDatabaseURL
	}

	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	db := databaseOpenDB(*config)

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
