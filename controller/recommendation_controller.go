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
	recommendationAuthFailedMessage          = "authentication failed"
	recommendationInvalidInputMessage        = "invalid input"
	recommendationInternalServerErrorMessage = "internal server error"
)

type RecommendationService interface {
	UploadDocument(ctx context.Context, input service.UploadDocumentInput) (service.UploadDocumentOutput, error)
	CreateSubmission(ctx context.Context, input service.CreateSubmissionInput) (service.CreateSubmissionOutput, error)
	CreateProfileRecommendation(ctx context.Context, userID string, req dto.CreateProfileRecommendationRequest) (service.CreateRecommendationWorkflowOutput, error)
	CreateTranscriptRecommendation(ctx context.Context, userID string, req dto.CreateTranscriptRecommendationRequest) (service.CreateRecommendationWorkflowOutput, error)
	CreateCVRecommendation(ctx context.Context, userID string, req dto.CreateCVRecommendationRequest) (service.CreateRecommendationWorkflowOutput, error)
	GetSubmissionDetail(ctx context.Context, userID, submissionID string) (repository.SubmissionDetail, error)
}

type RecommendationController struct {
	recommendationService RecommendationService
}

func NewRecommendationController(recommendationService RecommendationService) *RecommendationController {
	return &RecommendationController{recommendationService: recommendationService}
}

func (c *RecommendationController) UploadDocument(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: recommendationAuthFailedMessage})
		return
	}

	documentType := ctx.PostForm("document_type")
	if documentType != string(model.DocumentTypeTranscript) && documentType != string(model.DocumentTypeCV) {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid document type"})
		return
	}

	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: recommendationInvalidInputMessage})
		return
	}

	output, err := c.recommendationService.UploadDocument(ctx.Request.Context(), service.UploadDocumentInput{
		UserID:       userID.(string),
		DocumentType: model.DocumentType(documentType),
		File:         file,
	})
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrUnauthorized):
			ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: recommendationAuthFailedMessage})
		case errors.Is(err, errs.ErrInvalidInput):
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: recommendationInvalidInputMessage})
		default:
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: recommendationInternalServerErrorMessage})
		}
		return
	}

	ctx.JSON(http.StatusCreated, dto.UploadRecommendationDocumentResponse{
		Document: dto.RecommendationDocumentResponse{
			DocumentID:       output.Document.DocumentID,
			OriginalFilename: output.Document.OriginalFilename,
			PublicURL:        output.Document.PublicURL,
			MIMEType:         output.Document.MIMEType,
			SizeBytes:        output.Document.SizeBytes,
			DocumentType:     string(output.Document.DocumentType),
			UploadedAt:       output.Document.UploadedAt.UTC().Format(time.RFC3339),
		},
	})
}

func (c *RecommendationController) CreateSubmission(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: recommendationAuthFailedMessage})
		return
	}

	var req dto.CreateRecommendationSubmissionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: recommendationInvalidInputMessage})
		return
	}

	output, err := c.recommendationService.CreateSubmission(ctx.Request.Context(), service.CreateSubmissionInput{
		UserID:               userID.(string),
		TranscriptDocumentID: req.TranscriptDocumentID,
		CVDocumentID:         req.CVDocumentID,
		Preferences:          req.Preferences,
	})
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrUnauthorized):
			ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: recommendationAuthFailedMessage})
		case errors.Is(err, errs.ErrDocumentNotFound):
			ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "document not found"})
		case errors.Is(err, errs.ErrNoDocumentProvided), errors.Is(err, errs.ErrInvalidInput):
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: recommendationInvalidInputMessage})
		default:
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: recommendationInternalServerErrorMessage})
		}
		return
	}

	ctx.JSON(http.StatusCreated, dto.CreateRecommendationSubmissionResponse{
		SubmissionID: output.SubmissionID,
		Status:       string(output.Status),
		ResultSetID:  output.ResultSetID,
	})
}

func (c *RecommendationController) CreateProfileRecommendation(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: recommendationAuthFailedMessage})
		return
	}

	var req dto.CreateProfileRecommendationRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: recommendationInvalidInputMessage})
		return
	}

	output, err := c.recommendationService.CreateProfileRecommendation(ctx.Request.Context(), userID.(string), req)
	if err != nil {
		c.writeRecommendationWorkflowError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, dto.CreateRecommendationWorkflowResponse{
		SubmissionID: output.SubmissionID,
		Status:       string(output.Status),
		ResultSetID:  output.ResultSetID,
		Result:       output.Result,
	})
}

func (c *RecommendationController) CreateTranscriptRecommendation(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: recommendationAuthFailedMessage})
		return
	}

	var req dto.CreateTranscriptRecommendationRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: recommendationInvalidInputMessage})
		return
	}

	output, err := c.recommendationService.CreateTranscriptRecommendation(ctx.Request.Context(), userID.(string), req)
	if err != nil {
		c.writeRecommendationWorkflowError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, dto.CreateRecommendationWorkflowResponse{
		SubmissionID: output.SubmissionID,
		Status:       string(output.Status),
		ResultSetID:  output.ResultSetID,
		Result:       output.Result,
	})
}

func (c *RecommendationController) CreateCVRecommendation(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: recommendationAuthFailedMessage})
		return
	}

	var req dto.CreateCVRecommendationRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: recommendationInvalidInputMessage})
		return
	}

	output, err := c.recommendationService.CreateCVRecommendation(ctx.Request.Context(), userID.(string), req)
	if err != nil {
		c.writeRecommendationWorkflowError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, dto.CreateRecommendationWorkflowResponse{
		SubmissionID: output.SubmissionID,
		Status:       string(output.Status),
		ResultSetID:  output.ResultSetID,
		Result:       output.Result,
	})
}

func (c *RecommendationController) GetSubmissionDetail(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: recommendationAuthFailedMessage})
		return
	}

	submissionID := ctx.Param("id")
	if _, err := uuid.Parse(submissionID); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid id format"})
		return
	}

	detail, err := c.recommendationService.GetSubmissionDetail(ctx.Request.Context(), userID.(string), submissionID)
	if err != nil {
		c.writeGetSubmissionDetailError(ctx, err)
		return
	}

	response := dto.RecommendationSubmissionDetailResponse{
		SubmissionID: detail.Submission.RecSubmissionID,
		Status:       string(detail.Submission.Status),
		CreatedAt:    detail.Submission.CreatedAt.UTC().Format(time.RFC3339),
		Documents:    toRecommendationDocumentResponses(detail.Documents),
		Preferences:  toRecommendationPreferenceResponses(detail.Preferences),
		LatestResult: toRecommendationResultSetResponse(detail.LatestResultSet, detail.Results),
	}
	if detail.Submission.SubmittedAt != nil {
		response.SubmittedAt = detail.Submission.SubmittedAt.UTC().Format(time.RFC3339)
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *RecommendationController) writeGetSubmissionDetailError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, errs.ErrUnauthorized):
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: recommendationAuthFailedMessage})
	case errors.Is(err, errs.ErrSubmissionNotFound):
		ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "submission not found"})
	default:
		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: recommendationInternalServerErrorMessage})
	}
}

func (c *RecommendationController) writeRecommendationWorkflowError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, errs.ErrUnauthorized):
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: recommendationAuthFailedMessage})
	case errors.Is(err, errs.ErrInvalidInput), errors.Is(err, errs.ErrNoDocumentProvided):
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: recommendationInvalidInputMessage})
	case errors.Is(err, errs.ErrExternalService):
		ctx.JSON(http.StatusBadGateway, dto.ErrorResponse{Error: "external service error"})
	default:
		ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: recommendationInternalServerErrorMessage})
	}
}

func toRecommendationDocumentResponses(items []model.Document) []dto.RecommendationDocumentResponse {
	responses := make([]dto.RecommendationDocumentResponse, 0, len(items))
	for _, doc := range items {
		responses = append(responses, dto.RecommendationDocumentResponse{
			DocumentID:       doc.DocumentID,
			OriginalFilename: doc.OriginalFilename,
			PublicURL:        doc.PublicURL,
			MIMEType:         doc.MIMEType,
			SizeBytes:        doc.SizeBytes,
			DocumentType:     string(doc.DocumentType),
			UploadedAt:       doc.UploadedAt.UTC().Format(time.RFC3339),
		})
	}
	return responses
}

func toRecommendationPreferenceResponses(items []model.RecommendationPreference) []dto.RecommendationPreferenceResponse {
	responses := make([]dto.RecommendationPreferenceResponse, 0, len(items))
	for _, pref := range items {
		responses = append(responses, dto.RecommendationPreferenceResponse{
			PrefKey:   pref.PreferenceKey,
			PrefValue: pref.PreferenceValue,
		})
	}
	return responses
}

func toRecommendationResultSetResponse(set *model.RecommendationResultSet, rows []model.RecommendationResult) *dto.RecommendationResultSetResponse {
	if set == nil {
		return nil
	}

	resultResponses := make([]dto.RecommendationResultResponse, 0, len(rows))
	for _, row := range rows {
		resultResponses = append(resultResponses, dto.RecommendationResultResponse{
			RankNo:            row.RankNo,
			UniversityName:    row.UniversityName,
			ProgramName:       row.ProgramName,
			Country:           row.Country,
			FitScore:          row.FitScore,
			FitLevel:          row.FitLevel,
			Overview:          row.Overview,
			WhyThisUniversity: row.WhyThisUniversity,
			WhyThisProgram:    row.WhyThisProgram,
			ReasonSummary:     row.ReasonSummary,
			Pros:              decodeJSONStringArray(row.ProsJSON),
			Cons:              decodeJSONStringArray(row.ConsJSON),
		})
	}

	return &dto.RecommendationResultSetResponse{
		ResultSetID: set.ResultSetID,
		VersionNo:   set.VersionNo,
		GeneratedAt: set.GeneratedAt.UTC().Format(time.RFC3339),
		Results:     resultResponses,
	}
}

func decodeJSONStringArray(raw string) []string {
	result := make([]string, 0)
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return []string{}
	}
	return result
}
