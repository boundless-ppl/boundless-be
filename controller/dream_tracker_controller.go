package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/middleware"
	"boundless-be/model"
	"boundless-be/presenter"
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
	ListDreamTrackers(ctx context.Context, userID string) ([]repository.DreamTrackerDetail, error)
	GetGroupedDreamTrackers(ctx context.Context, userID string, selectedDreamTrackerID *string, includeDefaultDetail bool) (service.GroupedDreamTrackersOutput, error)
	GetDreamTrackerDashboardSummary(ctx context.Context, userID string) (service.DreamTrackerDashboardSummary, error)
	GetDreamTrackerDetail(ctx context.Context, userID, dreamTrackerID string) (repository.DreamTrackerDetail, error)
	GetDocumentDetail(ctx context.Context, userID, documentID string) (model.Document, error)
	UploadDreamRequirementDocument(ctx context.Context, input service.UploadDreamRequirementDocumentInput) (service.UploadDreamRequirementDocumentOutput, int, error)
	SubmitDreamRequirement(ctx context.Context, input service.SubmitDreamRequirementInput) (service.SubmitDreamRequirementOutput, error)
}

type DreamTrackerController struct {
	dreamTrackerService DreamTrackerService
	presenter           presenter.DreamTrackerPresenter
}

func NewDreamTrackerController(dreamTrackerService DreamTrackerService) *DreamTrackerController {
	return &DreamTrackerController{
		dreamTrackerService: dreamTrackerService,
		presenter:           presenter.NewDreamTrackerPresenter(),
	}
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

func (c *DreamTrackerController) ListDreamTrackers(ctx *gin.Context) {
	userID, ok := c.requireUserID(ctx)
	if !ok {
		return
	}

	items, err := c.dreamTrackerService.ListDreamTrackers(ctx.Request.Context(), userID)
	if err != nil {
		c.writeGetDreamTrackerError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, c.presenter.PresentTrackers(items))
}

func (c *DreamTrackerController) GetDreamTrackerDashboardSummary(ctx *gin.Context) {
	userID, ok := c.requireUserID(ctx)
	if !ok {
		return
	}

	summary, err := c.dreamTrackerService.GetDreamTrackerDashboardSummary(ctx.Request.Context(), userID)
	if err != nil {
		c.writeGetDreamTrackerError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, dto.DreamTrackerDashboardSummaryResponse{
		TotalApplications: summary.TotalTrackers,
		IncompleteCount:   summary.IncompleteTrackers,
		CompletedCount:    summary.CompletedTrackers,
		DeadlineNearCount: summary.NearDeadlineTrackers,
	})
}

func (c *DreamTrackerController) GetGroupedDreamTrackers(ctx *gin.Context) {
	userID, ok := c.requireUserID(ctx)
	if !ok {
		return
	}

	includeDefaultDetail, err := parseOptionalBool(ctx.Query("include_default_detail"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: dreamTrackerInvalidInputMessage})
		return
	}

	var selectedID *string
	if value := ctx.Query("selected_dream_tracker_id"); value != "" {
		if _, err := uuid.Parse(value); err != nil {
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: dreamTrackerInvalidIDFormatMessage})
			return
		}
		selectedID = &value
	}

	grouped, err := c.dreamTrackerService.GetGroupedDreamTrackers(ctx.Request.Context(), userID, selectedID, includeDefaultDetail)
	if err != nil {
		c.writeGetDreamTrackerError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, c.presentGroupedDreamTrackers(grouped))
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

	ctx.JSON(http.StatusOK, c.presenter.PresentTracker(detail))
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

func (c *DreamTrackerController) UploadDreamRequirementDocument(ctx *gin.Context) {
	userID, ok := c.requireUserID(ctx)
	if !ok {
		return
	}

	requirementID := ctx.Param("id")
	if _, err := uuid.Parse(requirementID); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: dreamTrackerInvalidIDFormatMessage})
		return
	}

	reuseIfExists := true
	if raw := ctx.PostForm("reuse_if_exists"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: dreamTrackerInvalidInputMessage})
			return
		}
		reuseIfExists = parsed
	}

	file, _ := ctx.FormFile("file")
	output, statusCode, err := c.dreamTrackerService.UploadDreamRequirementDocument(ctx.Request.Context(), service.UploadDreamRequirementDocumentInput{
		UserID:           userID,
		DreamReqStatusID: requirementID,
		DocumentType:     ctx.PostForm("document_type"),
		ReuseIfExists:    reuseIfExists,
		File:             file,
	})
	if err != nil {
		c.writeUploadDreamRequirementDocumentError(ctx, err)
		return
	}

	ctx.JSON(statusCode, c.presenter.PresentUploadRequirementDocument(output))
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

	ctx.JSON(http.StatusOK, c.presenter.PresentSubmitRequirement(output))
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

func (c *DreamTrackerController) writeUploadDreamRequirementDocumentError(ctx *gin.Context, err error) {
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

func isValidUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func parseOptionalBool(value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	return strconv.ParseBool(value)
}

func (c *DreamTrackerController) presentGroupedDreamTrackers(output service.GroupedDreamTrackersOutput) dto.DreamTrackerGroupedResponse {
	response := dto.DreamTrackerGroupedResponse{
		DefaultSelectedDreamTrackerID: output.DefaultSelectedDreamTrackerID,
		Universities:                  make([]dto.DreamTrackerUniversityGroupResponse, 0, len(output.Universities)),
		Fundings:                      make([]dto.DreamTrackerFundingGroupResponse, 0, len(output.Fundings)),
	}
	for _, group := range output.Universities {
		items := make([]dto.DreamTrackerGroupItemResponse, 0, len(group.Items))
		for _, item := range group.Items {
			items = append(items, dto.DreamTrackerGroupItemResponse{
				DreamTrackerID:       item.DreamTrackerID,
				Title:                item.Title,
				ProgramName:          item.ProgramName,
				AdmissionName:        item.AdmissionName,
				UniversityName:       item.UniversityName,
				Status:               string(item.Status),
				StatusLabel:          item.StatusLabel,
				CompletionPercentage: item.CompletionPercentage,
				IsSelected:           item.IsSelected,
			})
		}
		response.Universities = append(response.Universities, dto.DreamTrackerUniversityGroupResponse{
			UniversityID:   group.UniversityID,
			UniversityName: group.UniversityName,
			Items:          items,
		})
	}
	for _, group := range output.Fundings {
		items := make([]dto.DreamTrackerGroupItemResponse, 0, len(group.Items))
		for _, item := range group.Items {
			items = append(items, dto.DreamTrackerGroupItemResponse{
				DreamTrackerID:       item.DreamTrackerID,
				Title:                item.Title,
				ProgramName:          item.ProgramName,
				AdmissionName:        item.AdmissionName,
				UniversityName:       item.UniversityName,
				Status:               string(item.Status),
				StatusLabel:          item.StatusLabel,
				CompletionPercentage: item.CompletionPercentage,
				IsSelected:           item.IsSelected,
			})
		}
		response.Fundings = append(response.Fundings, dto.DreamTrackerFundingGroupResponse{
			FundingID:   group.FundingID,
			FundingName: group.FundingName,
			Items:       items,
		})
	}
	if output.DefaultDetail != nil {
		presented := c.presenter.PresentTracker(*output.DefaultDetail)
		response.DefaultDetail = &presented
	}
	return response
}
