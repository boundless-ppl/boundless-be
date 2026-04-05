package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"boundless-be/errs"
	"boundless-be/model"
)

type UniversityRepository interface {
	Create(ctx context.Context, u model.University) (model.University, error)
	FindAll(ctx context.Context) ([]model.University, error)
	FindByID(ctx context.Context, id string) (model.University, error)
	Update(ctx context.Context, u model.University) (model.University, error)
	Delete(ctx context.Context, id string) error
}

type DBUniversityRepository struct {
	db *sql.DB
}

func NewUniversityRepository(db *sql.DB) *DBUniversityRepository {
	return &DBUniversityRepository{db: db}
}

func (r *DBUniversityRepository) Create(ctx context.Context, u model.University) (model.University, error) {
	query := `
		INSERT INTO universities 
		(id, negara_id, nama, kota, tipe, deskripsi, website, ranking)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		u.ID,
		u.NegaraID,
		u.Nama,
		u.Kota,
		u.Tipe,
		u.Deskripsi,
		u.Website,
		u.Ranking,
	)

	if err != nil {
		return model.University{}, fmt.Errorf("create university: %w", err)
	}

	return u, nil
}

func (r *DBUniversityRepository) FindAll(ctx context.Context) ([]model.University, error) {
	query := `
		SELECT id, negara_id, nama, kota, tipe, deskripsi, website, ranking
		FROM universities
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("find all universities: %w", err)
	}
	defer rows.Close()

	var result []model.University
	for rows.Next() {
		var u model.University

		err := rows.Scan(
			&u.ID,
			&u.NegaraID,
			&u.Nama,
			&u.Kota,
			&u.Tipe,
			&u.Deskripsi,
			&u.Website,
			&u.Ranking,
		)

		if err != nil {
			return nil, fmt.Errorf("scan university: %w", err)
		}

		result = append(result, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find all universities: %w", err)
	}

	return result, nil
}

func (r *DBUniversityRepository) FindByID(ctx context.Context, id string) (model.University, error) {
	query := `
		SELECT id, negara_id, nama, kota, tipe, deskripsi, website, ranking
		FROM universities
		WHERE id = $1
	`

	var u model.University
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID,
		&u.NegaraID,
		&u.Nama,
		&u.Kota,
		&u.Tipe,
		&u.Deskripsi,
		&u.Website,
		&u.Ranking,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.University{}, errs.ErrUniversityNotFound
		}
		return model.University{}, fmt.Errorf("find university by id: %w", err)
	}

	return u, nil
}

func (r *DBUniversityRepository) Update(ctx context.Context, u model.University) (model.University, error) {
	query := `
		UPDATE universities
		SET negara_id=$2, nama=$3, kota=$4, tipe=$5, deskripsi=$6, website=$7, ranking=$8, updated_at=NOW()
		WHERE id=$1
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		u.ID,
		u.NegaraID,
		u.Nama,
		u.Kota,
		u.Tipe,
		u.Deskripsi,
		u.Website,
		u.Ranking,
	)

	if err != nil {
		return model.University{}, fmt.Errorf("update university: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return model.University{}, fmt.Errorf("check affected rows: %w", err)
	}

	if affected == 0 {
		return model.University{}, errs.ErrUniversityNotFound
	}

	return u, nil
}

func (r *DBUniversityRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM universities WHERE id=$1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete university: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check affected rows: %w", err)
	}

	if affected == 0 {
		return errs.ErrUniversityNotFound
	}

	return nil
}
