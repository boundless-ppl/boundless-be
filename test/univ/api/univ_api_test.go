package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"boundless-be/api"
	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/model"

	"github.com/google/uuid"
)

type testUniversityRepo struct {
	data map[string]model.University
}

func TestMain(m *testing.M) {
	os.Setenv("AUTH_SECRET", "test-secret")
	os.Exit(m.Run())
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
		return model.University{}, errs.ErrUniversityNotFound
	}
	return u, nil
}

func (r *testUniversityRepo) Update(ctx context.Context, u model.University) (model.University, error) {
	if _, ok := r.data[u.ID]; !ok {
		return model.University{}, errs.ErrUniversityNotFound
	}
	r.data[u.ID] = u
	return u, nil
}

func (r *testUniversityRepo) Delete(ctx context.Context, id string) error {
	if _, ok := r.data[id]; !ok {
		return errs.ErrUniversityNotFound
	}
	delete(r.data, id)
	return nil
}

func TestGetAllUniversitiesApi(t *testing.T) {
	repo := newTestUniversityRepo()

	id := uuid.New().String()
	repo.data[id] = model.University{
		ID:        id,
		NegaraID:  "INA",
		Nama:      "ITB",
		Kota:      "Bandung",
		Tipe:      model.NATIONAL,
		Deskripsi: "Institut Teknologi Bandung",
		Website:   "https://itb.ac.id",
	}

	handler := api.NewHandler(api.Dependencies{
		UnivRepo: repo,
	})

	req := httptest.NewRequest(http.MethodGet, "/universities", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, rec.Code)
	}
}

func TestCreateUniversityApi(t *testing.T) {
	repo := newTestUniversityRepo()

	handler := api.NewHandler(api.Dependencies{
		UnivRepo: repo,
	})

	body, _ := json.Marshal(dto.CreateUniversityRequest{
		NegaraID:  "INA",
		Nama:      "ITB",
		Kota:      "Bandung",
		Tipe:      string(model.NATIONAL),
		Deskripsi: "Institut Teknologi Bandung",
		Website:   "https://itb.ac.id",
	})

	req := httptest.NewRequest(http.MethodPost, "/universities", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d got %d", http.StatusCreated, rec.Code)
	}
}

func TestUpdateUniversityApi(t *testing.T) {
	repo := newTestUniversityRepo()

	id := uuid.New().String()

	repo.data[id] = model.University{
		ID:        id,
		NegaraID:  "INA",
		Nama:      "ITB",
		Kota:      "Bandung",
		Tipe:      model.NATIONAL,
		Deskripsi: "Institut Teknologi Bandung",
		Website:   "https://itb.ac.id",
	}

	handler := api.NewHandler(api.Dependencies{
		UnivRepo: repo,
	})

	body, _ := json.Marshal(dto.UpdateUniversityRequest{
		NegaraID:  "INA",
		Nama:      "ITB",
		Kota:      "Bandung UPDATE",
		Tipe:      string(model.PRIVATE),
		Deskripsi: "Institut Teknologi Bandung",
		Website:   "https://itb.ac.id",
	})

	req := httptest.NewRequest(http.MethodPatch, "/universities/"+id, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, rec.Code)
	}
}

func TestDeleteUniversityApi(t *testing.T) {
	repo := newTestUniversityRepo()

	id := uuid.New().String()

	repo.data[id] = model.University{
		ID:        id,
		NegaraID:  "INA",
		Nama:      "ITB",
		Kota:      "Bandung",
		Tipe:      model.NATIONAL,
		Deskripsi: "Institut Teknologi Bandung",
		Website:   "https://itb.ac.id",
	}

	handler := api.NewHandler(api.Dependencies{
		UnivRepo: repo,
	})

	req := httptest.NewRequest(http.MethodDelete, "/universities/"+id, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d got %d", http.StatusNoContent, rec.Code)
	}
}
