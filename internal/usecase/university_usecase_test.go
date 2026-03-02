package usecase_test

import (
	"boundless-be/internal/domain"
	"boundless-be/internal/usecase"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockUniversityRepository struct {
	CreateFn   func(u *domain.University) error
	FindAllFn  func() ([]domain.University, error)
	FindByIDFn func(id string) (*domain.University, error)
	UpdateFn   func(u *domain.University) error
	DeleteFn   func(id string) error
}

func (m *MockUniversityRepository) Create(u *domain.University) error {
	return m.CreateFn(u)
}

func (m *MockUniversityRepository) FindAll() ([]domain.University, error) {
	return m.FindAllFn()
}

func (m *MockUniversityRepository) FindByID(id string) (*domain.University, error) {
	return m.FindByIDFn(id)
}

func (m *MockUniversityRepository) Update(u *domain.University) error {
	return m.UpdateFn(u)
}

func (m *MockUniversityRepository) Delete(id string) error {
	return m.DeleteFn(id)
}

// UNIVERSITY USECASE TESTS
func TestCreateUniversity_ShouldSuccess(t *testing.T) {
	mockRepo := &MockUniversityRepository{
		CreateFn: func(u *domain.University) error {
			return nil
		},
	}

	uc := usecase.NewUniversityUsecase(mockRepo)

	u := &domain.University{
		NegaraID:  "INA",
		Nama:      "ITB",
		Kota:      "Bandung",
		Tipe:      domain.NATIONAL,
		Deskripsi: "Institut Teknologi Bandung",
		Website:   "https://www.itb.ac.id",
		Ranking:   func(i int) *int { return &i }(255),
	}

	err := uc.CreateUniversity(u)

	assert.NoError(t, err)
}

func TestCreateUniversity_ShouldFailIfAnyRequiredFieldEmpty(t *testing.T) {

	tests := []struct {
		name  string
		input domain.University
	}{
		{
			name: "nama kosong",
			input: domain.University{
				NegaraID:  "INA",
				Nama:      "",
				Kota:      "Bandung",
				Tipe:      domain.NATIONAL,
				Deskripsi: "Institut Teknologi Bandung",
				Website:   "https://www.itb.ac.id",
			},
		},
		{
			name: "kota kosong",
			input: domain.University{
				NegaraID:  "INA",
				Nama:      "ITB",
				Kota:      "",
				Tipe:      domain.NATIONAL,
				Deskripsi: "Institut Teknologi Bandung",
				Website:   "https://www.itb.ac.id",
			},
		},
		{
			name: "tipe kosong",
			input: domain.University{
				NegaraID:  "INA",
				Nama:      "ITB",
				Kota:      "Bandung",
				Tipe:      "",
				Deskripsi: "Institut Teknologi Bandung",
				Website:   "https://www.itb.ac.id",
			},
		},
		{
			name: "deskripsi kosong",
			input: domain.University{
				NegaraID:  "INA",
				Nama:      "ITB",
				Kota:      "Bandung",
				Tipe:      domain.NATIONAL,
				Deskripsi: "",
				Website:   "https://www.itb.ac.id",
			},
		},
		{
			name: "website kosong",
			input: domain.University{
				NegaraID:  "INA",
				Nama:      "ITB",
				Kota:      "Bandung",
				Tipe:      domain.NATIONAL,
				Deskripsi: "Institut Teknologi Bandung",
				Website:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mockRepo := &MockUniversityRepository{
				CreateFn: func(u *domain.University) error {
					t.Fatal("Repo should not be called when validation fails")
					return nil
				},
			}

			uc := usecase.NewUniversityUsecase(mockRepo)

			err := uc.CreateUniversity(&tt.input)

			assert.Error(t, err)
		})
	}
}

func TestGetAllUniversities_ShouldReturnData(t *testing.T) {
	expected := []domain.University{
		{
			ID:        "1",
			NegaraID:  "INA",
			Nama:      "ITB",
			Kota:      "Bandung",
			Tipe:      domain.NATIONAL,
			Deskripsi: "Institut Teknologi Bandung",
			Website:   "https://www.itb.ac.id",
			Ranking:   func(i int) *int { return &i }(255),
		},
	}

	mockRepo := &MockUniversityRepository{
		FindAllFn: func() ([]domain.University, error) {
			return expected, nil
		},
	}

	uc := usecase.NewUniversityUsecase(mockRepo)

	result, err := uc.GetAllUniversities()

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestGetAllUniversities_ShouldReturnEmptySliceIfNoData(t *testing.T) {
	mockRepo := &MockUniversityRepository{
		FindAllFn: func() ([]domain.University, error) {
			return []domain.University{}, nil
		},
	}

	uc := usecase.NewUniversityUsecase(mockRepo)

	result, err := uc.GetAllUniversities()

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}

func TestGetUniversityByID_ShouldReturnDataIfExists(t *testing.T) {
	expected := &domain.University{
		ID:        "1",
		NegaraID:  "INA",
		Nama:      "ITB",
		Kota:      "Bandung",
		Tipe:      domain.NATIONAL,
		Deskripsi: "Institut Teknologi Bandung",
		Website:   "https://www.itb.ac.id",
		Ranking:   func(i int) *int { return &i }(255),
	}

	mockRepo := &MockUniversityRepository{
		FindByIDFn: func(id string) (*domain.University, error) {
			return expected, nil
		},
	}

	uc := usecase.NewUniversityUsecase(mockRepo)

	result, err := uc.GetUniversityByID("1")

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestGetUniversityByID_ShouldReturnErrorIfNotFound(t *testing.T) {
	mockRepo := &MockUniversityRepository{
		FindByIDFn: func(id string) (*domain.University, error) {
			return nil, nil
		},
	}

	uc := usecase.NewUniversityUsecase(mockRepo)

	result, err := uc.GetUniversityByID("999")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestUpdateUniversity_ShouldFailIfNotFound(t *testing.T) {
	mockRepo := &MockUniversityRepository{
		FindByIDFn: func(id string) (*domain.University, error) {
			return nil, nil
		},
		UpdateFn: func(u *domain.University) error {
			t.Fatal("Update should not be called if not found.")
			return nil
		},
	}

	uc := usecase.NewUniversityUsecase(mockRepo)

	u := &domain.University{
		ID:        "999",
		NegaraID:  "INA",
		Nama:      "Random Updated",
		Kota:      "Bandung",
		Tipe:      domain.NATIONAL,
		Deskripsi: "Random Updated",
		Website:   "https://www.itb.ac.id/updated",
	}

	err := uc.UpdateUniversity(u)

	assert.Error(t, err)
}

func TestUpdateUniversity_ShouldPatchFields(t *testing.T) {
	existing := &domain.University{
		ID:        "1",
		NegaraID:  "INA",
		Nama:      "ITB",
		Kota:      "Bandung",
		Tipe:      domain.NATIONAL,
		Deskripsi: "Institut Teknologi Bandung",
		Website:   "https://www.itb.ac.id",
	}

	mockRepo := &MockUniversityRepository{
		FindByIDFn: func(id string) (*domain.University, error) {
			return existing, nil
		},
		UpdateFn: func(u *domain.University) error {
			// pastikan patch terjadi
			assert.Equal(t, "ITB Updated", u.Nama)
			assert.Equal(t, "Bandung", u.Kota) // tidak berubah
			return nil
		},
	}

	uc := usecase.NewUniversityUsecase(mockRepo)

	updatePayload := &domain.University{
		ID:   "1",
		Nama: "ITB Updated",
	}

	err := uc.UpdateUniversity(updatePayload)

	assert.NoError(t, err)
}

func TestDeleteUniversity_ShouldSuccess(t *testing.T) {
	called := false

	mockRepo := &MockUniversityRepository{
		FindByIDFn: func(id string) (*domain.University, error) {
			return &domain.University{ID: "1"}, nil
		},
		DeleteFn: func(id string) error {
			called = true
			return nil
		},
	}

	uc := usecase.NewUniversityUsecase(mockRepo)

	err := uc.DeleteUniversity("1")

	assert.NoError(t, err)
	assert.True(t, called)
}

func TestDeleteUniversity_ShouldFailIfNotFound(t *testing.T) {
	mockRepo := &MockUniversityRepository{
		FindByIDFn: func(id string) (*domain.University, error) {
			return nil, nil
		},
		DeleteFn: func(id string) error {
			t.Fatal("Delete should not be called if not found.")
			return nil
		},
	}

	uc := usecase.NewUniversityUsecase(mockRepo)

	err := uc.DeleteUniversity("1")

	assert.Error(t, err)
}
