package controller

import (
	"context"
	"database/sql"
	"errors"
	"mime/multipart"
	"net/http"

	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/middleware"

	"github.com/gin-gonic/gin"
)

type DreamTrackerService interface {
	GetSummary(ctx context.Context, userID string) (dto.DreamTrackerDashboardSummaryResponse, error)
	GetGrouped(ctx context.Context, userID string, includeDefaultDetail bool, selectedDreamTrackerID string) (dto.DreamTrackerGroupedResponse, error)
	GetByID(ctx context.Context, userID, dreamTrackerID string) (dto.DreamTrackerItemResponse, error)
	Create(ctx context.Context, userID string, req dto.CreateDreamTrackerRequest) (dto.CreateDreamTrackerResponse, error)
	UploadRequirementDocument(ctx context.Context, userID, requirementStatusID, documentType string, file *multipart.FileHeader) (dto.SubmitRequirementResponse, error)
}

type DreamTrackerController struct {
	service DreamTrackerService
}

func NewDreamTrackerController(service DreamTrackerService) *DreamTrackerController {
	return &DreamTrackerController{service: service}
}

func (c *DreamTrackerController) GetSummary(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}
	result, err := c.service.GetSummary(ctx.Request.Context(), userID.(string))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *DreamTrackerController) GetGrouped(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}
	result, err := c.service.GetGrouped(
		ctx.Request.Context(),
		userID.(string),
		ctx.Query("include_default_detail") == "true",
		ctx.Query("selected_dream_tracker_id"),
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *DreamTrackerController) GetByID(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}
	result, err := c.service.GetByID(ctx.Request.Context(), userID.(string), ctx.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "dream tracker not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		}
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *DreamTrackerController) Create(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}
	var req dto.CreateDreamTrackerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		return
	}
	result, err := c.service.Create(ctx.Request.Context(), userID.(string), req)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrInvalidInput):
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		default:
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		}
		return
	}
	ctx.JSON(http.StatusCreated, result)
}

func (c *DreamTrackerController) UploadRequirementDocument(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}
	documentType := ctx.PostForm("document_type")
	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		return
	}
	result, err := c.service.UploadRequirementDocument(ctx.Request.Context(), userID.(string), ctx.Param("id"), documentType, file)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrInvalidInput):
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		case errors.Is(err, sql.ErrNoRows):
			ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "requirement not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		}
		return
	}
	ctx.JSON(http.StatusCreated, result)
}
