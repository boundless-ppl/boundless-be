package service

import (
	"context"

	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"

	"github.com/google/uuid"
)

type UniversityService struct {
	universityRepo repository.UniversityRepository
}

func NewUniversityService(repo repository.UniversityRepository) *UniversityService {
	return &UniversityService{
		universityRepo: repo,
	}
}

func (s *UniversityService) CreateUniversity(ctx context.Context, req dto.CreateUniversityRequest) (model.University, error) {
	if req.Nama == "" ||
		req.Kota == "" ||
		req.Tipe == "" ||
		req.Deskripsi == "" ||
		req.Website == "" {
		return model.University{}, errs.ErrInvalidInput
	}

	u := model.University{
		ID:        uuid.NewString(),
		NegaraID:  req.NegaraID,
		Nama:      req.Nama,
		Kota:      req.Kota,
		Tipe:      model.UniversityType(req.Tipe),
		Deskripsi: req.Deskripsi,
		Website:   req.Website,
		Ranking:   req.Ranking,
	}

	return s.universityRepo.Create(ctx, u)
}

func (s *UniversityService) GetAllUniversities(ctx context.Context) ([]model.University, error) {
	return s.universityRepo.FindAll(ctx)
}

func (s *UniversityService) GetUniversityByID(ctx context.Context, id string) (model.University, error) {
	data, err := s.universityRepo.FindByID(ctx, id)
	if err != nil {
		return model.University{}, err
	}

	return data, nil
}

func (s *UniversityService) UpdateUniversity(ctx context.Context, id string, req dto.UpdateUniversityRequest) (model.University, error) {
	existing, err := s.universityRepo.FindByID(ctx, id)
	if err != nil {
		return model.University{}, err
	}

	if req.Nama != "" {
		existing.Nama = req.Nama
	}

	if req.NegaraID != "" {
		existing.NegaraID = req.NegaraID
	}

	if req.Kota != "" {
		existing.Kota = req.Kota
	}

	if req.Tipe != "" {
		existing.Tipe = model.UniversityType(req.Tipe)
	}

	if req.Deskripsi != "" {
		existing.Deskripsi = req.Deskripsi
	}

	if req.Website != "" {
		existing.Website = req.Website
	}

	if req.Ranking != nil {
		existing.Ranking = req.Ranking
	}

	return s.universityRepo.Update(ctx, existing)
}

func (s *UniversityService) DeleteUniversity(ctx context.Context, id string) error {
	_, err := s.universityRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	return s.universityRepo.Delete(ctx, id)
}
