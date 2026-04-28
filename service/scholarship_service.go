package service

import (
	"context"

	"boundless-be/dto"
	"boundless-be/repository"
)

type ScholarshipService struct {
	scholarshipRepo repository.ScholarshipRepository
}

func NewScholarshipService(repo repository.ScholarshipRepository) *ScholarshipService {
	return &ScholarshipService{scholarshipRepo: repo}
}

func (s *ScholarshipService) ListScholarships(ctx context.Context, query dto.ScholarshipListQuery) (repository.ScholarshipListResult, error) {
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 12
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return s.scholarshipRepo.List(ctx, repository.ScholarshipListFilter{
		Page:           page,
		PageSize:       pageSize,
		Search:         query.Search,
		TipePembiayaan: query.TipePembiayaan,
		Negara:         query.Negara,
	})
}

func (s *ScholarshipService) GetScholarshipByID(ctx context.Context, id string) (repository.ScholarshipItem, error) {
	return s.scholarshipRepo.FindByID(ctx, id)
}
