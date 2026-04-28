package service_test

import (
	"context"
	"errors"
	"testing"

	"boundless-be/dto"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"
)

type fakeScholarshipRepo struct {
	lastFilter repository.ScholarshipListFilter
	listOut    repository.ScholarshipListResult
	listErr    error

	findID  string
	findOut repository.ScholarshipItem
	findErr error
}

func (f *fakeScholarshipRepo) List(ctx context.Context, filter repository.ScholarshipListFilter) (repository.ScholarshipListResult, error) {
	f.lastFilter = filter
	return f.listOut, f.listErr
}

func (f *fakeScholarshipRepo) FindByID(ctx context.Context, id string) (repository.ScholarshipItem, error) {
	f.findID = id
	return f.findOut, f.findErr
}

func TestScholarshipServiceList_DefaultPagination(t *testing.T) {
	repo := &fakeScholarshipRepo{}
	svc := service.NewScholarshipService(repo)

	_, _ = svc.ListScholarships(context.Background(), dto.ScholarshipListQuery{})

	if repo.lastFilter.Page != 1 {
		t.Fatalf("expected default page 1, got %d", repo.lastFilter.Page)
	}
	if repo.lastFilter.PageSize != 12 {
		t.Fatalf("expected default page_size 12, got %d", repo.lastFilter.PageSize)
	}
}

func TestScholarshipServiceList_CapPageSizeTo100(t *testing.T) {
	repo := &fakeScholarshipRepo{}
	svc := service.NewScholarshipService(repo)

	_, _ = svc.ListScholarships(context.Background(), dto.ScholarshipListQuery{Page: 2, PageSize: 999})

	if repo.lastFilter.Page != 2 {
		t.Fatalf("expected page 2, got %d", repo.lastFilter.Page)
	}
	if repo.lastFilter.PageSize != 100 {
		t.Fatalf("expected capped page_size 100, got %d", repo.lastFilter.PageSize)
	}
}

func TestScholarshipServiceList_ForwardFilters(t *testing.T) {
	repo := &fakeScholarshipRepo{}
	svc := service.NewScholarshipService(repo)

	query := dto.ScholarshipListQuery{
		Page:           3,
		PageSize:       20,
		Search:         "LPDP",
		TipePembiayaan: "Penuh",
		Negara:         "Indonesia",
	}

	_, _ = svc.ListScholarships(context.Background(), query)

	if repo.lastFilter.Search != "LPDP" {
		t.Fatalf("expected search LPDP, got %s", repo.lastFilter.Search)
	}
	if repo.lastFilter.TipePembiayaan != "Penuh" {
		t.Fatalf("expected tipe_pembiayaan Penuh, got %s", repo.lastFilter.TipePembiayaan)
	}
	if repo.lastFilter.Negara != "Indonesia" {
		t.Fatalf("expected negara Indonesia, got %s", repo.lastFilter.Negara)
	}
}

func TestScholarshipServiceGetByID_ForwardsID(t *testing.T) {
	repo := &fakeScholarshipRepo{findOut: repository.ScholarshipItem{Funding: model.FundingOption{FundingID: "abc"}}}
	svc := service.NewScholarshipService(repo)

	_, err := svc.GetScholarshipByID(context.Background(), "abc")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.findID != "abc" {
		t.Fatalf("expected id abc, got %s", repo.findID)
	}
}

func TestScholarshipServiceGetByID_PropagatesError(t *testing.T) {
	repo := &fakeScholarshipRepo{findErr: errors.New("db error")}
	svc := service.NewScholarshipService(repo)

	_, err := svc.GetScholarshipByID(context.Background(), "abc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
