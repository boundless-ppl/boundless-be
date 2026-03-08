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

type RecommendationService interface {
	UploadDocument(ctx context.Context, input service.UploadDocumentInput) (service.UploadDocumentOutput, error)
	CreateSubmission(ctx context.Context, input service.CreateSubmissionInput) (service.CreateSubmissionOutput, error)
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
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}

	documentType := ctx.PostForm("document_type")
	if documentType != string(model.DocumentTypeTranscript) && documentType != string(model.DocumentTypeCV) {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid document type"})
		return
	}

	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
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
			ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		case errors.Is(err, errs.ErrInvalidInput):
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		default:
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
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
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}

	var req dto.CreateRecommendationSubmissionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
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
			ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		case errors.Is(err, errs.ErrDocumentNotFound):
			ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "document not found"})
		case errors.Is(err, errs.ErrNoDocumentProvided), errors.Is(err, errs.ErrInvalidInput):
			ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid input"})
		default:
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		}
		return
	}

	ctx.JSON(http.StatusCreated, dto.CreateRecommendationSubmissionResponse{
		SubmissionID: output.SubmissionID,
		Status:       string(output.Status),
		ResultSetID:  output.ResultSetID,
	})
}

func (c *RecommendationController) GetSubmissionDetail(ctx *gin.Context) {
	userID, ok := ctx.Get(middleware.UserIDContextKey)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		return
	}

	submissionID := ctx.Param("id")
	if _, err := uuid.Parse(submissionID); err != nil {
		ctx.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid id format"})
		return
	}

	detail, err := c.recommendationService.GetSubmissionDetail(ctx.Request.Context(), userID.(string), submissionID)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrUnauthorized):
			ctx.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "authentication failed"})
		case errors.Is(err, errs.ErrSubmissionNotFound):
			ctx.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "submission not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal server error"})
		}
		return
	}

	docResponses := make([]dto.RecommendationDocumentResponse, 0, len(detail.Documents))
	for _, doc := range detail.Documents {
		docResponses = append(docResponses, dto.RecommendationDocumentResponse{
			DocumentID:       doc.DocumentID,
			OriginalFilename: doc.OriginalFilename,
			PublicURL:        doc.PublicURL,
			MIMEType:         doc.MIMEType,
			SizeBytes:        doc.SizeBytes,
			DocumentType:     string(doc.DocumentType),
			UploadedAt:       doc.UploadedAt.UTC().Format(time.RFC3339),
		})
	}

	prefResponses := make([]dto.RecommendationPreferenceResponse, 0, len(detail.Preferences))
	for _, pref := range detail.Preferences {
		prefResponses = append(prefResponses, dto.RecommendationPreferenceResponse{
			PrefKey:   pref.PreferenceKey,
			PrefValue: pref.PreferenceValue,
		})
	}

	var latestResult *dto.RecommendationResultSetResponse
	if detail.LatestResultSet != nil {
		resultResponses := make([]dto.RecommendationResultResponse, 0, len(detail.Results))
		for _, row := range detail.Results {
			pros := make([]string, 0)
			cons := make([]string, 0)
			if err := json.Unmarshal([]byte(row.ProsJSON), &pros); err != nil {
				pros = []string{}
			}
			if err := json.Unmarshal([]byte(row.ConsJSON), &cons); err != nil {
				cons = []string{}
			}
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
				Pros:              pros,
				Cons:              cons,
			})
		}

		latestResult = &dto.RecommendationResultSetResponse{
			ResultSetID: detail.LatestResultSet.ResultSetID,
			VersionNo:   detail.LatestResultSet.VersionNo,
			GeneratedAt: detail.LatestResultSet.GeneratedAt.UTC().Format(time.RFC3339),
			Results:     resultResponses,
		}
	}

	response := dto.RecommendationSubmissionDetailResponse{
		SubmissionID: detail.Submission.RecSubmissionID,
		Status:       string(detail.Submission.Status),
		CreatedAt:    detail.Submission.CreatedAt.UTC().Format(time.RFC3339),
		Documents:    docResponses,
		Preferences:  prefResponses,
		LatestResult: latestResult,
	}
	if detail.Submission.SubmittedAt != nil {
		response.SubmittedAt = detail.Submission.SubmittedAt.UTC().Format(time.RFC3339)
	}

	ctx.JSON(http.StatusOK, response)
}
