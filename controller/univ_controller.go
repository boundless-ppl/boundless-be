package controller

import (
	"errors"
	"net/http"

	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UniversityController struct {
	service *service.UniversityService
}

func NewUniversityController(s *service.UniversityService) *UniversityController {
	return &UniversityController{service: s}
}

func toUniversityResponse(u model.University) dto.UniversityResponse {
	return dto.UniversityResponse{
		ID:        u.ID,
		NegaraID:  u.NegaraID,
		Nama:      u.Nama,
		Kota:      u.Kota,
		Tipe:      string(u.Tipe),
		Deskripsi: u.Deskripsi,
		Website:   u.Website,
		Ranking:   u.Ranking,
	}
}

func (c *UniversityController) Create(ctx *gin.Context) {
	var req dto.CreateUniversityRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		return
	}

	result, err := c.service.CreateUniversity(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errs.ErrInvalidInput) {
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
			return
		}

		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal server error",
		})
		return
	}

	ctx.JSON(http.StatusCreated, toUniversityResponse(result))
}

func (c *UniversityController) GetAll(ctx *gin.Context) {
	result, err := c.service.GetAllUniversities(ctx.Request.Context())

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal server error",
		})
		return
	}

	responses := make([]dto.UniversityResponse, 0, len(result))

	for _, u := range result {
		responses = append(responses, toUniversityResponse(u))
	}

	ctx.JSON(http.StatusOK, responses)
}

func (c *UniversityController) GetByID(ctx *gin.Context) {
	id := ctx.Param("id")

	if _, err := uuid.Parse(id); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "invalid id format",
		})
		return
	}

	result, err := c.service.GetUniversityByID(ctx.Request.Context(), id)
	if err != nil {
		if errors.Is(err, errs.ErrUniversityNotFound) {
			ctx.JSON(http.StatusNotFound, dto.ErrorResponse{
				Error: "university not found",
			})
			return
		}

		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal server error",
		})
		return
	}

	ctx.JSON(http.StatusOK, toUniversityResponse(result))
}

func (c *UniversityController) Update(ctx *gin.Context) {
	id := ctx.Param("id")

	if _, err := uuid.Parse(id); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "invalid id format",
		})
		return
	}

	var req dto.UpdateUniversityRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		return
	}

	result, err := c.service.UpdateUniversity(ctx.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, errs.ErrUniversityNotFound) {
			ctx.JSON(http.StatusNotFound, dto.ErrorResponse{
				Error: "university not found",
			})
			return
		}

		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal server error",
		})
		return
	}

	ctx.JSON(http.StatusOK, toUniversityResponse(result))
}

func (c *UniversityController) Delete(ctx *gin.Context) {
	id := ctx.Param("id")

	if _, err := uuid.Parse(id); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "invalid id format",
		})
		return
	}

	err := c.service.DeleteUniversity(ctx.Request.Context(), id)
	if err != nil {
		if errors.Is(err, errs.ErrUniversityNotFound) {
			ctx.JSON(http.StatusNotFound, dto.ErrorResponse{
				Error: "university not found",
			})
			return
		}

		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "internal server error",
		})
		return
	}

	ctx.Status(http.StatusNoContent)
}
