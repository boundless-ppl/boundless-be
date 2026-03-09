package controller_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"boundless-be/controller"
	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeUniversityRepository struct {
	createResult model.University
	createErr    error

	findAllResult []model.University
	findAllErr    error

	findByIDResult model.University
	findByIDErr    error

	updateResult model.University
	updateErr    error

	deleteErr error
}

func (f *fakeUniversityRepository) Create(ctx context.Context, u model.University) (model.University, error) {
	return f.createResult, f.createErr
}

func (f *fakeUniversityRepository) FindAll(ctx context.Context) ([]model.University, error) {
	return f.findAllResult, f.findAllErr
}

func (f *fakeUniversityRepository) FindByID(ctx context.Context, id string) (model.University, error) {
	return f.findByIDResult, f.findByIDErr
}

func (f *fakeUniversityRepository) Update(ctx context.Context, u model.University) (model.University, error) {
	return f.updateResult, f.updateErr
}

func (f *fakeUniversityRepository) Delete(ctx context.Context, id string) error {
	return f.deleteErr
}

func setupRouter(repo *fakeUniversityRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)

	svc := service.NewUniversityService(repo)
	c := controller.NewUniversityController(svc)

	router := gin.New()

	router.POST("/universities", c.Create)
	router.GET("/universities", c.GetAll)
	router.GET("/universities/:id", c.GetByID)
	router.PATCH("/universities/:id", c.Update)
	router.DELETE("/universities/:id", c.Delete)

	return router
}

func TestCreateUniversitySuccessController(t *testing.T) {
	repo := &fakeUniversityRepository{
		createResult: model.University{
			ID:        uuid.NewString(),
			NegaraID:  "INA",
			Nama:      "ITB",
			Kota:      "Bandung",
			Tipe:      model.NATIONAL,
			Deskripsi: "Institut Teknologi Bandung",
			Website:   "https://itb.ac.id",
		},
	}

	router := setupRouter(repo)

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
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d got %d", http.StatusCreated, rec.Code)
	}
}

func TestCreateUniversityInvalidBodyController(t *testing.T) {
	repo := &fakeUniversityRepository{}
	router := setupRouter(repo)

	req := httptest.NewRequest(http.MethodPost, "/universities", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestCreateUniversityInvalidInputController(t *testing.T) {
	repo := &fakeUniversityRepository{}
	router := setupRouter(repo)

	body, _ := json.Marshal(dto.CreateUniversityRequest{})

	req := httptest.NewRequest(http.MethodPost, "/universities", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetAllUniversitiesController(t *testing.T) {
	repo := &fakeUniversityRepository{
		findAllResult: []model.University{
			{ID: "1", Nama: "ITB"},
			{ID: "2", Nama: "UI"},
		},
	}

	router := setupRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/universities", nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, rec.Code)
	}
}

func TestGetUniversityByIDSuccessController(t *testing.T) {
	id := uuid.NewString()

	repo := &fakeUniversityRepository{
		findByIDResult: model.University{
			ID:   id,
			Nama: "ITB",
		},
	}

	router := setupRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/universities/"+id, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, rec.Code)
	}
}

func TestGetUniversityByIDInvalidIDController(t *testing.T) {
	repo := &fakeUniversityRepository{}
	router := setupRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/universities/invalid-id", nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetUniversityByIDNotFoundController(t *testing.T) {
	id := uuid.NewString()

	repo := &fakeUniversityRepository{
		findByIDErr: errs.ErrUniversityNotFound,
	}

	router := setupRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/universities/"+id, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d got %d", http.StatusNotFound, rec.Code)
	}
}

func TestUpdateUniversitySuccessController(t *testing.T) {
	id := uuid.NewString()

	repo := &fakeUniversityRepository{
		findByIDResult: model.University{
			ID:        id,
			NegaraID:  "INA",
			Nama:      "ITB",
			Kota:      "Bandung",
			Tipe:      model.NATIONAL,
			Deskripsi: "Institut Teknologi Bandung",
			Website:   "https://itb.ac.id",
		},
		updateResult: model.University{
			ID:        id,
			NegaraID:  "INA",
			Nama:      "ITB Updated",
			Kota:      "Bandung",
			Tipe:      model.NATIONAL,
			Deskripsi: "Institut Teknologi Bandung",
			Website:   "https://itb.ac.id",
		},
	}

	router := setupRouter(repo)

	body, _ := json.Marshal(dto.UpdateUniversityRequest{
		Nama: "ITB Updated",
	})

	req := httptest.NewRequest(http.MethodPatch, "/universities/"+id, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Log(rec.Body.String())
		t.Fatalf("expected %d got %d", http.StatusOK, rec.Code)
	}
}

func TestDeleteUniversitySuccessController(t *testing.T) {
	id := uuid.NewString()

	repo := &fakeUniversityRepository{
		findByIDResult: model.University{
			ID: id,
		},
	}

	router := setupRouter(repo)

	req := httptest.NewRequest(http.MethodDelete, "/universities/"+id, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d got %d", http.StatusNoContent, rec.Code)
	}
}

func TestDeleteUniversityNotFoundController(t *testing.T) {
	id := uuid.NewString()

	repo := &fakeUniversityRepository{
		findByIDErr: errs.ErrUniversityNotFound,
	}

	router := setupRouter(repo)

	req := httptest.NewRequest(http.MethodDelete, "/universities/"+id, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d got %d", http.StatusNotFound, rec.Code)
	}
}
