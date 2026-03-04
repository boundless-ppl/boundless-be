package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"boundless-be/model"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrEmailExists  = errors.New("email already exists")
	ErrUserNotFound = errors.New("user not found")
)

type UserRepository interface {
	Create(ctx context.Context, user model.User) (model.User, error)
	FindByEmail(ctx context.Context, email string) (model.User, error)
	FindByID(ctx context.Context, userID string) (model.User, error)
	Update(ctx context.Context, user model.User) error
}

type DBUserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *DBUserRepository {
	return &DBUserRepository{db: db}
}

func (r *DBUserRepository) Create(ctx context.Context, user model.User) (model.User, error) {
	query := `
		INSERT INTO users (
			user_id, nama_lengkap, role, email, password_hash, created_at,
			failed_login_count, first_failed_at, locked_until
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`

	email := strings.ToLower(strings.TrimSpace(user.Email))
	_, err := r.db.ExecContext(
		ctx,
		query,
		user.UserID,
		user.NamaLengkap,
		user.Role,
		email,
		user.PasswordHash,
		user.CreatedAt,
		user.FailedLoginCount,
		nullTime(user.FirstFailedAt),
		nullTime(user.LockedUntil),
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return model.User{}, ErrEmailExists
		}
		return model.User{}, err
	}

	user.Email = email
	return user, nil
}

func (r *DBUserRepository) FindByEmail(ctx context.Context, email string) (model.User, error) {
	query := `
		SELECT user_id, nama_lengkap, role, email, password_hash, created_at,
		       failed_login_count, first_failed_at, locked_until
		FROM users WHERE email = $1
	`
	row := r.db.QueryRowContext(ctx, query, strings.ToLower(strings.TrimSpace(email)))
	return scanUser(row)
}

func (r *DBUserRepository) FindByID(ctx context.Context, userID string) (model.User, error) {
	query := `
		SELECT user_id, nama_lengkap, role, email, password_hash, created_at,
		       failed_login_count, first_failed_at, locked_until
		FROM users WHERE user_id = $1
	`
	row := r.db.QueryRowContext(ctx, query, userID)
	return scanUser(row)
}

func (r *DBUserRepository) Update(ctx context.Context, user model.User) error {
	query := `
		UPDATE users
		SET nama_lengkap = $2,
		    role = $3,
		    email = $4,
		    password_hash = $5,
		    failed_login_count = $6,
		    first_failed_at = $7,
		    locked_until = $8
		WHERE user_id = $1
	`
	result, err := r.db.ExecContext(
		ctx,
		query,
		user.UserID,
		user.NamaLengkap,
		user.Role,
		strings.ToLower(strings.TrimSpace(user.Email)),
		user.PasswordHash,
		user.FailedLoginCount,
		nullTime(user.FirstFailedAt),
		nullTime(user.LockedUntil),
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrUserNotFound
	}
	return nil
}

type userScanner interface {
	Scan(dest ...any) error
}

func scanUser(row userScanner) (model.User, error) {
	var user model.User
	var firstFailed sql.NullTime
	var lockedUntil sql.NullTime

	err := row.Scan(
		&user.UserID,
		&user.NamaLengkap,
		&user.Role,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.FailedLoginCount,
		&firstFailed,
		&lockedUntil,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, err
	}

	if firstFailed.Valid {
		user.FirstFailedAt = firstFailed.Time
	}
	if lockedUntil.Valid {
		user.LockedUntil = lockedUntil.Time
	}
	return user, nil
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
