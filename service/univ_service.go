package service

import (
	"context"
	"errors"

	"boundless-be/model"
	"boundless-be/repository"
)

var (
	ErrUniversityNotFound = errors.New("university not found")
)

type UniversityService struct {
	universityRepo repository.UniversityRepository
}

func NewUniversityService(repo repository.UniversityRepository) *UniversityService {
	return &UniversityService{
		universityRepo: repo,
	}
}

func (s *UniversityService) CreateUniversity(ctx context.Context, input model.University) (model.University, error) {
	if input.NegaraID == "" ||
		input.Nama == "" ||
		input.Kota == "" ||
		input.Tipe == "" ||
		input.Deskripsi == "" ||
		input.Website == "" {

		return model.University{}, ErrInvalidInput
	}

	return s.universityRepo.Create(ctx, input)
}

func (s *UniversityService) GetAllUniversities(ctx context.Context) ([]model.University, error) {
	return s.universityRepo.FindAll(ctx)
}

func (s *UniversityService) GetUniversityByID(ctx context.Context, id string) (model.University, error) {
	data, err := s.universityRepo.FindByID(ctx, id)
	if err != nil {
		return model.University{}, err
	}

	if data.ID == "" {
		return model.University{}, ErrUniversityNotFound
	}

	return data, nil
}

func (s *UniversityService) UpdateUniversity(ctx context.Context, input model.University) (model.University, error) {
	existing, err := s.universityRepo.FindByID(ctx, input.ID)
	if err != nil {
		return model.University{}, err
	}

	if existing.ID == "" {
		return model.University{}, ErrUniversityNotFound
	}

	if input.Nama != "" {
		existing.Nama = input.Nama
	}
	if input.NegaraID != "" {
		existing.NegaraID = input.NegaraID
	}
	if input.Kota != "" {
		existing.Kota = input.Kota
	}
	if input.Tipe != "" {
		existing.Tipe = input.Tipe
	}
	if input.Deskripsi != "" {
		existing.Deskripsi = input.Deskripsi
	}
	if input.Website != "" {
		existing.Website = input.Website
	}
	if input.Ranking != nil {
		existing.Ranking = input.Ranking
	}

	return s.universityRepo.Update(ctx, existing)
}

func (s *UniversityService) DeleteUniversity(ctx context.Context, id string) error {
	existing, err := s.universityRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if existing.ID == "" {
		return ErrUniversityNotFound
	}

	return s.universityRepo.Delete(ctx, id)
}
