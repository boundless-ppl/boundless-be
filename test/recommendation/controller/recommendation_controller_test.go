package controller_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"boundless-be/controller"
	"boundless-be/errs"
	"boundless-be/middleware"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeRecommendationService struct {
	uploadOutput service.UploadDocumentOutput
	uploadErr    error

	createOutput service.CreateSubmissionOutput
	createErr    error
	detailOutput repository.SubmissionDetail
	detailErr    error
}

func (f *fakeRecommendationService) UploadDocument(ctx context.Context, input service.UploadDocumentInput) (service.UploadDocumentOutput, error) {
	return f.uploadOutput, f.uploadErr
}

func (f *fakeRecommendationService) CreateSubmission(ctx context.Context, input service.CreateSubmissionInput) (service.CreateSubmissionOutput, error) {
	return f.createOutput, f.createErr
}

func (f *fakeRecommendationService) GetSubmissionDetail(ctx context.Context, userID, submissionID string) (repository.SubmissionDetail, error) {
	return f.detailOutput, f.detailErr
}

func setupRecommendationRouter(svc *fakeRecommendationService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	ctrl := controller.NewRecommendationController(svc)
	router := gin.New()

	router.POST("/recommendations/submissions", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.CreateSubmission)
	router.POST("/recommendations/documents", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.UploadDocument)
	router.GET("/recommendations/submissions/:id", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.GetSubmissionDetail)

	return router
}

func TestUploadRecommendationDocumentSuccessController(t *testing.T) {
	docID := uuid.NewString()

	svc := &fakeRecommendationService{
		uploadOutput: service.UploadDocumentOutput{
			Document: model.Document{
				DocumentID:       docID,
				OriginalFilename: "transcript.pdf",
				PublicURL:        "http://example/transcript.pdf",
				MIMEType:         "application/pdf",
				SizeBytes:        123,
				DocumentType:     model.DocumentTypeTranscript,
				UploadedAt:       time.Now().UTC(),
			},
		},
	}

	router := setupRecommendationRouter(svc)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("document_type", "transcript"); err != nil {
		t.Fatal(err)
	}

	fw, err := writer.CreateFormFile("file", "transcript.pdf")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := fw.Write([]byte("dummy")); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/recommendations/documents",
		body,
	)

	req.Header.Set("Content-Type", writer.FormDataContentType())

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d got %d", http.StatusCreated, rec.Code)
	}

	var resp struct {
		Document struct {
			DocumentID string `json:"document_id"`
		} `json:"document"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response body: %v", err)
	}

	if resp.Document.DocumentID != docID {
		t.Fatalf("unexpected document id")
	}
}

func TestCreateRecommendationSubmissionSuccessController(t *testing.T) {
	submissionID := uuid.NewString()
	svc := &fakeRecommendationService{}
	svc.createOutput = service.CreateSubmissionOutput{SubmissionID: submissionID, Status: model.RecommendationStatusDraft}
	router := setupRecommendationRouter(svc)

	body := []byte(fmt.Sprintf(`{"transcript_document_id":"%s","preferences":[{"pref_key":"country","pref_value":"Japan"}]}`, uuid.NewString()))

	req := httptest.NewRequest(http.MethodPost, "/recommendations/submissions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d got %d", http.StatusCreated, rec.Code)
	}
}

func TestGetRecommendationSubmissionNotFoundController(t *testing.T) {
	svc := &fakeRecommendationService{detailErr: errs.ErrSubmissionNotFound}
	router := setupRecommendationRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/recommendations/submissions/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d got %d", http.StatusNotFound, rec.Code)
	}
}

func TestGetRecommendationSubmissionSuccessController(t *testing.T) {
	now := time.Now().UTC()
	submissionID := uuid.NewString()
	svc := &fakeRecommendationService{
		detailOutput: repository.SubmissionDetail{
			Submission: model.RecommendationSubmission{
				RecSubmissionID: submissionID,
				Status:          model.RecommendationStatusCompleted,
				CreatedAt:       now,
			},
			Documents: []model.Document{{
				DocumentID:       uuid.NewString(),
				OriginalFilename: "transcript.pdf",
				PublicURL:        "http://example/transcript.pdf",
				MIMEType:         "application/pdf",
				SizeBytes:        123,
				DocumentType:     model.DocumentTypeTranscript,
				UploadedAt:       now,
			}},
			Preferences: []model.RecommendationPreference{{
				PreferenceKey:   "country",
				PreferenceValue: "Japan",
			}},
			LatestResultSet: &model.RecommendationResultSet{
				ResultSetID: uuid.NewString(),
				VersionNo:   1,
				GeneratedAt: now,
			},
			Results: []model.RecommendationResult{{
				RankNo:         1,
				UniversityName: "University A",
				ProgramName:    "CS",
				Country:        "Japan",
				FitScore:       90,
				FitLevel:       "cocok",
				Overview:       "overview",
				ProsJSON:       `["pro1"]`,
				ConsJSON:       `["con1"]`,
			}},
		},
	}
	router := setupRecommendationRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/recommendations/submissions/"+submissionID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json body, got %v", err)
	}
}

func TestUploadRecommendationDocumentInvalidTypeController(t *testing.T) {
	svc := &fakeRecommendationService{}
	router := setupRecommendationRouter(svc)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	writer.WriteField("document_type", "invalid")

	fw, _ := writer.CreateFormFile("file", "file.pdf")
	fw.Write([]byte("dummy"))
	writer.Close()

	req := httptest.NewRequest(
		http.MethodPost,
		"/recommendations/documents",
		body,
	)

	req.Header.Set("Content-Type", writer.FormDataContentType())

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}
