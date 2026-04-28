package controller_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"boundless-be/controller"
	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

type fakeScholarshipRepository struct {
	listOut repository.ScholarshipListResult
	listErr error
	findOut repository.ScholarshipItem
	findErr error
}

func (f *fakeScholarshipRepository) List(ctx context.Context, filter repository.ScholarshipListFilter) (repository.ScholarshipListResult, error) {
	return f.listOut, f.listErr
}

func (f *fakeScholarshipRepository) FindByID(ctx context.Context, id string) (repository.ScholarshipItem, error) {
	return f.findOut, f.findErr
}

func setupScholarshipRouter(repo *fakeScholarshipRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := service.NewScholarshipService(repo)
	c := controller.NewScholarshipController(svc)
	r := gin.New()
	r.GET("/scholarships", c.List)
	r.GET("/scholarships/:id", c.GetByID)
	return r
}

func TestScholarshipListSuccessController(t *testing.T) {
	desc := "beasiswa penuh"
	repo := &fakeScholarshipRepository{
		listOut: repository.ScholarshipListResult{
			Data: []repository.ScholarshipItem{{
				Funding: model.FundingOption{
					FundingID:      "fund-1",
					NamaBeasiswa:   "LPDP",
					Provider:       "Kemenkeu",
					Deskripsi:      &desc,
					TipePembiayaan: model.FundingTypeScholarship,
					Website:        "https://lpdp.go.id",
				},
				Deadline:      "2026-12-31",
				Negara:        "Indonesia",
				Persyaratan:   []string{"WNI"},
				Benefit:       []string{"Tuition"},
				LinkDaftarURL: "https://lpdp.go.id",
				IsActive:      true,
			}},
			Total:     1,
			Page:      1,
			PageSize:  12,
			TotalPage: 1,
		},
	}

	r := setupScholarshipRouter(repo)
	req := httptest.NewRequest(http.MethodGet, "/scholarships?page=1&page_size=12", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, rec.Code)
	}

	var out dto.ScholarshipListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(out.Data) != 1 {
		t.Fatalf("expected 1 data, got %d", len(out.Data))
	}
	if out.Data[0].TipePembiayaan != "Penuh" {
		t.Fatalf("expected tipe_pembiayaan Penuh, got %s", out.Data[0].TipePembiayaan)
	}
}

func TestScholarshipListInternalServerErrorController(t *testing.T) {
	repo := &fakeScholarshipRepository{listErr: context.DeadlineExceeded}
	r := setupScholarshipRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/scholarships", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestScholarshipGetByIDNotFoundController(t *testing.T) {
	repo := &fakeScholarshipRepository{findErr: errs.ErrScholarshipNotFound}
	r := setupScholarshipRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/scholarships/fund-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d got %d", http.StatusNotFound, rec.Code)
	}
}

func TestScholarshipGetByIDSuccessController(t *testing.T) {
	repo := &fakeScholarshipRepository{
		findOut: repository.ScholarshipItem{
			Funding: model.FundingOption{
				FundingID:      "fund-1",
				NamaBeasiswa:   "LPDP",
				Provider:       "Kemenkeu",
				TipePembiayaan: model.FundingTypeLoan,
				Website:        "https://lpdp.go.id",
			},
			Deadline:      "2026-12-31",
			Negara:        "Indonesia",
			LinkDaftarURL: "https://lpdp.go.id",
			IsActive:      true,
		},
	}
	r := setupScholarshipRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/scholarships/fund-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, rec.Code)
	}

	var out dto.ScholarshipResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if out.TipePembiayaan != "Parsial" {
		t.Fatalf("expected tipe_pembiayaan Parsial, got %s", out.TipePembiayaan)
	}
}
