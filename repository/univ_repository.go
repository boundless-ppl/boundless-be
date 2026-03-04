package repository

import (
	"context"

	"boundless-be/model"
)

type UniversityRepository interface {
	Create(ctx context.Context, u model.University) (model.University, error)
	FindAll(ctx context.Context) ([]model.University, error)
	FindByID(ctx context.Context, id string) (model.University, error)
	Update(ctx context.Context, u model.University) (model.University, error)
	Delete(ctx context.Context, id string) error
}
