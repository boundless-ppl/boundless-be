package usecase

import (
	"boundless-be/internal/domain"
	"boundless-be/internal/errors"
	"boundless-be/internal/repository"
)

type UniversityUsecase struct {
	repo repository.UniversityRepository
}

func NewUniversityUsecase(r repository.UniversityRepository) *UniversityUsecase {
	return &UniversityUsecase{repo: r}
}

func (u *UniversityUsecase) CreateUniversity(input *domain.University) error {
	// validasi required field
	if input.NegaraID == "" ||
		input.Nama == "" ||
		input.Kota == "" ||
		input.Tipe == "" ||
		input.Deskripsi == "" ||
		input.Website == "" {

		return errors.NewBadRequest("Required field is empty.")
	}

	err := u.repo.Create(input)
	if err != nil {
		return err
	}

	return nil
}

func (u *UniversityUsecase) GetAllUniversities() ([]domain.University, error) {
	return u.repo.FindAll()
}

func (u *UniversityUsecase) GetUniversityByID(id string) (*domain.University, error) {
	data, err := u.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, errors.NewNotFound("University not found.")
	}

	return data, nil
}

func (u *UniversityUsecase) UpdateUniversity(input *domain.University) error {
	// find existing data
	existing, err := u.repo.FindByID(input.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.NewNotFound("University not found.")
	}

	// patch
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

	return u.repo.Update(existing)
}
func (u *UniversityUsecase) DeleteUniversity(id string) error {
	// find existing data
	existing, err := u.repo.FindByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.NewNotFound("University not found.")
	}

	return u.repo.Delete(id)
}
