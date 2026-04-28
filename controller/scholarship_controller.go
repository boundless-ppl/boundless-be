package controller

import (
	"errors"
	"net/http"

	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
)

type ScholarshipController struct {
	service *service.ScholarshipService
}

func NewScholarshipController(s *service.ScholarshipService) *ScholarshipController {
	return &ScholarshipController{service: s}
}

func toScholarshipUniversityResponse(u model.University) dto.ScholarshipUniversityResponse {
	tipe := string(u.Tipe)
	return dto.ScholarshipUniversityResponse{
		UniversityID: u.ID,
		Nama:         u.Nama,
		Kota:         u.Kota,
		Negara:       u.NegaraID,
		Ranking:      u.Ranking,
		Website:      &u.Website,
		Tipe:         &tipe,
		Deskripsi:    &u.Deskripsi,
	}
}

func toScholarshipResponse(s repository.ScholarshipItem) dto.ScholarshipResponse {
	universities := make([]dto.ScholarshipUniversityResponse, 0, len(s.Universitas))
	for _, u := range s.Universitas {
		universities = append(universities, toScholarshipUniversityResponse(u))
	}

	desc := ""
	if s.Funding.Deskripsi != nil {
		desc = *s.Funding.Deskripsi
	}

	return dto.ScholarshipResponse{
		ID:              s.Funding.FundingID,
		Nama:            s.Funding.NamaBeasiswa,
		Provider:        s.Funding.Provider,
		Deskripsi:       desc,
		Persyaratan:     s.Persyaratan,
		Benefit:         s.Benefit,
		Deadline:        s.Deadline,
		LinkPendaftaran: s.LinkDaftarURL,
		TipePembiayaan:  fundingTypeLabel(s.Funding.TipePembiayaan),
		Negara:          s.Negara,
		IsActive:        s.IsActive,
		Universitas:     universities,
	}
}

func (c *ScholarshipController) List(ctx *gin.Context) {
	var query dto.ScholarshipListQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid query"})
		return
	}

	result, err := c.service.ListScholarships(ctx.Request.Context(), query)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		return
	}

	data := make([]dto.ScholarshipResponse, 0, len(result.Data))
	for _, item := range result.Data {
		data = append(data, toScholarshipResponse(item))
	}

	ctx.JSON(http.StatusOK, dto.ScholarshipListResponse{
		Data:      data,
		Total:     result.Total,
		Page:      result.Page,
		PageSize:  result.PageSize,
		TotalPage: result.TotalPage,
	})
}

func (c *ScholarshipController) GetByID(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid id format"})
		return
	}

	result, err := c.service.GetScholarshipByID(ctx.Request.Context(), id)
	if err != nil {
		if errors.Is(err, errs.ErrScholarshipNotFound) {
			ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "scholarship not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		return
	}

	ctx.JSON(http.StatusOK, toScholarshipResponse(result))
}

func fundingTypeLabel(t model.FundingType) string {
	switch t {
	case model.FundingTypeSelfFunded, model.FundingTypeLoan:
		return "Parsial"
	default:
		return "Penuh"
	}
}
