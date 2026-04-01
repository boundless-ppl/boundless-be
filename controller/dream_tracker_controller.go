package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/middleware"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	dreamTrackerAuthFailedMessage          = "authentication failed"
	dreamTrackerInvalidInputMessage        = "invalid input"
	dreamTrackerInternalServerErrorMessage = "internal server error"
	dreamTrackerInvalidIDFormatMessage     = "invalid id format"
)

type DreamTrackerService interface {
	CreateDreamTracker(ctx context.Context, input service.CreateDreamTrackerInput) (service.CreateDreamTrackerOutput, error)
	GetDreamTrackerDetail(ctx context.Context, userID, dreamTrackerID string) (repository.DreamTrackerDetail, error)
	GetDocumentDetail(ctx context.Context, userID, documentID string) (model.Document, error)
	SubmitDreamRequirement(ctx context.Context, input service.SubmitDreamRequirementInput) (service.SubmitDreamRequirementOutput, error)
}

type DreamTrackerController struct {
	dreamTrackerService DreamTrackerService
}

func NewDreamTrackerController(dreamTrackerService DreamTrackerService) *DreamTrackerController {
	return &DreamTrackerController{dreamTrackerService: dreamTrackerService}
}

func (c *DreamTrackerController) CreateDreamTracker(ctx *gin.Context) {
	userID, ok := c.requireUserID(ctx)
	if !ok {
		return
	}

	var req dto.CreateDreamTrackerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: dreamTrackerInvalidInputMessage})
		return
	}

	output, err := c.dreamTrackerService.CreateDreamTracker(ctx.Request.Context(), service.CreateDreamTrackerInput{
		UserID:            userID,
		ProgramID:         req.ProgramID,
		AdmissionID:       req.AdmissionID,
		FundingID:         req.FundingID,
		Title:             req.Title,
		Status:            req.Status,
		SourceType:        req.SourceType,
		ReqSubmissionID:   req.ReqSubmissionID,
		SourceRecResultID: req.SourceRecResultID,
	})
	if err != nil {
		c.writeCreateDreamTrackerError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, dto.CreateDreamTrackerResponse{
		DreamTrackerID: output.DreamTracker.DreamTrackerID,
		Status:         string(output.DreamTracker.Status),
	})
}

func (c *DreamTrackerController) GetDreamTrackerDetail(ctx *gin.Context) {
	userID, ok := c.requireUserID(ctx)
	if !ok {
		return
	}

	dreamTrackerID := ctx.Param("id")
	if _, err := uuid.Parse(dreamTrackerID); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: dreamTrackerInvalidIDFormatMessage})
		return
	}

	detail, err := c.dreamTrackerService.GetDreamTrackerDetail(ctx.Request.Context(), userID, dreamTrackerID)
	if err != nil {
		c.writeGetDreamTrackerError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, dto.DreamTrackerResponse{
		DreamTrackerID:    detail.DreamTracker.DreamTrackerID,
		UserID:            detail.DreamTracker.UserID,
		ProgramID:         detail.DreamTracker.ProgramID,
		AdmissionID:       detail.DreamTracker.AdmissionID,
		FundingID:         detail.DreamTracker.FundingID,
		Title:             detail.DreamTracker.Title,
		Status:            string(detail.DreamTracker.Status),
		CreatedAt:         detail.DreamTracker.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         detail.DreamTracker.UpdatedAt.UTC().Format(time.RFC3339),
		SourceType:        detail.DreamTracker.SourceType,
		ReqSubmissionID:   detail.DreamTracker.ReqSubmissionID,
		SourceRecResultID: detail.DreamTracker.SourceRecResultID,
		Requirements:      toDreamRequirementStatusResponses(detail.Requirements),
	})
}

func (c *DreamTrackerController) GetDocumentDetail(ctx *gin.Context) {
	userID, ok := c.requireUserID(ctx)
	if !ok {
		return
	}

	documentID := ctx.Param("id")
	if _, err := uuid.Parse(documentID); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: dreamTrackerInvalidIDFormatMessage})
		return
	}

	doc, err := c.dreamTrackerService.GetDocumentDetail(ctx.Request.Context(), userID, documentID)
	if err != nil {
		c.writeGetDocumentError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, dto.DreamTrackerDocumentResponse{
		DocumentID:       doc.DocumentID,
		UserID:           doc.UserID,
		Nama:             doc.Nama,
		OriginalFilename: doc.OriginalFilename,
		DokumenURL:       doc.DokumenURL,
		PublicURL:        doc.PublicURL,
		MIMEType:         doc.MIMEType,
		SizeBytes:        doc.SizeBytes,
		DokumenSizeKB:    doc.DokumenSizeKB,
		DocumentType:     string(doc.DocumentType),
		UploadedAt:       doc.UploadedAt.UTC().Format(time.RFC3339),
	})
}

func (c *DreamTrackerController) SubmitDreamRequirement(ctx *gin.Context) {
	userID, ok := c.requireUserID(ctx)
	if !ok {
		return
	}

	requirementID := ctx.Param("id")
	if _, err := uuid.Parse(requirementID); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: dreamTrackerInvalidIDFormatMessage})
		return
	}

	var req dto.SubmitDreamRequirementRequest
	if err := ctx.ShouldBindJSON(&req); err != nil || !isValidUUID(req.DocumentID) {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: dreamTrackerInvalidInputMessage})
		return
	}

	output, err := c.dreamTrackerService.SubmitDreamRequirement(ctx.Request.Context(), service.SubmitDreamRequirementInput{
		UserID:           userID,
		DreamReqStatusID: requirementID,
		DocumentID:       req.DocumentID,
	})
	if err != nil {
		c.writeSubmitDreamRequirementError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, dto.SubmitDreamRequirementResponse{
		DreamReqStatusID: output.Requirement.DreamReqStatusID,
		DocumentID:       output.Requirement.DocumentID,
		Status:           string(output.Requirement.Status),
		AIStatus:         output.Requirement.AIStatus,
		AIMessages:       output.AIMessages,
	})
}

func (c *DreamTrackerController) requireUserID(ctx *gin.Context) (string, bool) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: dreamTrackerAuthFailedMessage})
		return "", false
	}
	return userID.(string), true
}

func (c *DreamTrackerController) writeCreateDreamTrackerError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, errs.ErrUnauthorized):
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: dreamTrackerAuthFailedMessage})
	case errors.Is(err, errs.ErrInvalidInput):
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: dreamTrackerInvalidInputMessage})
	default:
		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: dreamTrackerInternalServerErrorMessage})
	}
}

func (c *DreamTrackerController) writeGetDreamTrackerError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, errs.ErrUnauthorized):
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: dreamTrackerAuthFailedMessage})
	case errors.Is(err, errs.ErrInvalidInput):
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: dreamTrackerInvalidInputMessage})
	case errors.Is(err, errs.ErrDreamTrackerNotFound):
		ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "dream tracker not found"})
	default:
		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: dreamTrackerInternalServerErrorMessage})
	}
}

func (c *DreamTrackerController) writeGetDocumentError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, errs.ErrUnauthorized):
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: dreamTrackerAuthFailedMessage})
	case errors.Is(err, errs.ErrInvalidInput):
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: dreamTrackerInvalidInputMessage})
	case errors.Is(err, errs.ErrDocumentNotFound):
		ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "document not found"})
	default:
		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: dreamTrackerInternalServerErrorMessage})
	}
}

func (c *DreamTrackerController) writeSubmitDreamRequirementError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, errs.ErrUnauthorized):
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: dreamTrackerAuthFailedMessage})
	case errors.Is(err, errs.ErrInvalidInput):
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: dreamTrackerInvalidInputMessage})
	case errors.Is(err, errs.ErrDocumentNotFound):
		ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "document not found"})
	case errors.Is(err, errs.ErrDreamRequirementNotFound):
		ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "dream requirement not found"})
	default:
		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: dreamTrackerInternalServerErrorMessage})
	}
}

func toDreamRequirementStatusResponses(items []model.DreamRequirementStatus) []dto.DreamRequirementStatusResponse {
	requirements := make([]dto.DreamRequirementStatusResponse, 0, len(items))
	for _, item := range items {
		requirements = append(requirements, dto.DreamRequirementStatusResponse{
			DreamReqStatusID: item.DreamReqStatusID,
			DocumentID:       item.DocumentID,
			ReqCatalogID:     item.ReqCatalogID,
			Status:           string(item.Status),
			Notes:            item.Notes,
			AIStatus:         item.AIStatus,
			AIMessages:       decodeAIMessages(item.AIMessages),
			CreatedAt:        item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return requirements
}

func isValidUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func decodeAIMessages(raw *string) []string {
	if raw == nil || *raw == "" {
		return []string{}
	}

	var messages []string
	if err := json.Unmarshal([]byte(*raw), &messages); err == nil {
		return messages
	}

	return []string{*raw}
}
