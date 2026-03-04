package service_test

import (
	"boundless-be/model"
	"boundless-be/service"
	"context"
	"errors"
	"testing"
)

type testUniversityRepo struct {
	data map[string]model.University
}

func newTestUniversityRepo() *testUniversityRepo {
	return &testUniversityRepo{
		data: map[string]model.University{},
	}
}

func (r *testUniversityRepo) Create(ctx context.Context, u model.University) (model.University, error) {
	r.data[u.ID] = u
	return u, nil
}

func (r *testUniversityRepo) FindAll(ctx context.Context) ([]model.University, error) {
	var result []model.University
	for _, v := range r.data {
		result = append(result, v)
	}
	return result, nil
}

func (r *testUniversityRepo) FindByID(ctx context.Context, id string) (model.University, error) {
	u, ok := r.data[id]
	if !ok {
		return model.University{}, nil
	}
	return u, nil
}

func (r *testUniversityRepo) Update(ctx context.Context, u model.University) (model.University, error) {
	if _, ok := r.data[u.ID]; !ok {
		return model.University{}, nil
	}
	r.data[u.ID] = u
	return u, nil
}

func (r *testUniversityRepo) Delete(ctx context.Context, id string) error {
	if _, ok := r.data[id]; !ok {
		return nil
	}
	delete(r.data, id)
	return nil
}

func TestCreateUniversity_ShouldSuccess(t *testing.T) {
	repo := newTestUniversityRepo()
	svc := service.NewUniversityService(repo)

	input := model.University{
		ID:        "1",
		NegaraID:  "INA",
		Nama:      "ITB",
		Kota:      "Bandung",
		Tipe:      model.NATIONAL,
		Deskripsi: "Institut Teknologi Bandung",
		Website:   "https://itb.ac.id",
	}

	result, err := svc.CreateUniversity(context.Background(), input)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if result.ID != "1" {
		t.Fatal("expected university to be created")
	}
}

func TestCreateUniversity_ShouldFailIfAnyRequiredFieldEmpty(t *testing.T) {

	tests := []struct {
		name  string
		input model.University
	}{
		{
			name: "nama kosong",
			input: model.University{
				ID:        "1",
				NegaraID:  "INA",
				Nama:      "",
				Kota:      "Bandung",
				Tipe:      model.NATIONAL,
				Deskripsi: "Institut Teknologi Bandung",
				Website:   "https://itb.ac.id",
			},
		},
		{
			name: "kota kosong",
			input: model.University{
				ID:        "1",
				NegaraID:  "INA",
				Nama:      "ITB",
				Kota:      "",
				Tipe:      model.NATIONAL,
				Deskripsi: "Institut Teknologi Bandung",
				Website:   "https://itb.ac.id",
			},
		},
		{
			name: "tipe kosong",
			input: model.University{
				ID:        "1",
				NegaraID:  "INA",
				Nama:      "ITB",
				Kota:      "Bandung",
				Tipe:      "",
				Deskripsi: "Institut Teknologi Bandung",
				Website:   "https://itb.ac.id",
			},
		},
		{
			name: "deskripsi kosong",
			input: model.University{
				ID:        "1",
				NegaraID:  "INA",
				Nama:      "ITB",
				Kota:      "Bandung",
				Tipe:      model.NATIONAL,
				Deskripsi: "",
				Website:   "https://itb.ac.id",
			},
		},
		{
			name: "website kosong",
			input: model.University{
				ID:        "1",
				NegaraID:  "INA",
				Nama:      "ITB",
				Kota:      "Bandung",
				Tipe:      model.NATIONAL,
				Deskripsi: "Institut Teknologi Bandung",
				Website:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			repo := newTestUniversityRepo()
			svc := service.NewUniversityService(repo)

			_, err := svc.CreateUniversity(context.Background(), tt.input)

			if !errors.Is(err, service.ErrInvalidInput) {
				t.Fatalf("expected %v, got %v", service.ErrInvalidInput, err)
			}
		})
	}
}

func TestGetAllUniversities_ShouldReturnData(t *testing.T) {
	repo := newTestUniversityRepo()

	repo.data["1"] = model.University{
		ID:       "1",
		Nama:     "ITB",
		NegaraID: "INA",
		Kota:     "Bandung",
		Tipe:     model.NATIONAL,
	}

	svc := service.NewUniversityService(repo)

	result, err := svc.GetAllUniversities(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(result) != 1 {
		t.Fatal("expected 1 university")
	}
}

func TestGetUniversityByID_ShouldReturnDataIfExists(t *testing.T) {
	repo := newTestUniversityRepo()

	repo.data["1"] = model.University{
		ID:   "1",
		Nama: "ITB",
	}

	svc := service.NewUniversityService(repo)

	result, err := svc.GetUniversityByID(context.Background(), "1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if result.ID != "1" {
		t.Fatal("expected university to exist")
	}
}

func TestGetUniversityByID_ShouldReturnErrorIfNotFound(t *testing.T) {
	repo := newTestUniversityRepo()
	svc := service.NewUniversityService(repo)

	_, err := svc.GetUniversityByID(context.Background(), "999")

	if !errors.Is(err, service.ErrUniversityNotFound) {
		t.Fatalf("expected %v, got %v", service.ErrUniversityNotFound, err)
	}
}

func TestUpdateUniversity_ShouldFailIfNotFound(t *testing.T) {
	repo := newTestUniversityRepo()
	svc := service.NewUniversityService(repo)

	update := model.University{
		ID:   "999",
		Nama: "Random Updated",
	}

	_, err := svc.UpdateUniversity(context.Background(), update)

	if !errors.Is(err, service.ErrUniversityNotFound) {
		t.Fatalf("expected %v, got %v", service.ErrUniversityNotFound, err)
	}
}

func TestUpdateUniversity_ShouldPatchFields(t *testing.T) {
	repo := newTestUniversityRepo()

	repo.data["1"] = model.University{
		ID:   "1",
		Nama: "ITB",
		Kota: "Bandung",
	}

	svc := service.NewUniversityService(repo)

	updatePayload := model.University{
		ID:   "1",
		Nama: "ITB Updated",
	}

	updated, err := svc.UpdateUniversity(context.Background(), updatePayload)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if updated.Nama != "ITB Updated" {
		t.Fatal("expected name to be patched")
	}
	if updated.Kota != "Bandung" {
		t.Fatal("expected other fields unchanged")
	}
}

func TestDeleteUniversity_ShouldSuccess(t *testing.T) {
	repo := newTestUniversityRepo()

	repo.data["1"] = model.University{ID: "1"}

	svc := service.NewUniversityService(repo)

	err := svc.DeleteUniversity(context.Background(), "1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestDeleteUniversity_ShouldFailIfNotFound(t *testing.T) {
	repo := newTestUniversityRepo()
	svc := service.NewUniversityService(repo)

	err := svc.DeleteUniversity(context.Background(), "999")

	if !errors.Is(err, service.ErrUniversityNotFound) {
		t.Fatalf("expected %v, got %v", service.ErrUniversityNotFound, err)
	}
}
