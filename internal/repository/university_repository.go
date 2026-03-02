package repository

import "boundless-be/internal/domain"

type UniversityRepository interface {
	Create(u *domain.University) error
	FindAll() ([]domain.University, error)
	FindByID(id string) (*domain.University, error)
	Update(u *domain.University) error
	Delete(id string) error
}
