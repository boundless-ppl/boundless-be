package controller_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"boundless-be/controller"
	"boundless-be/dto"
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

	createOutput     service.CreateSubmissionOutput
	createErr        error
	profileOutput    service.CreateRecommendationWorkflowOutput
	profileErr       error
	transcriptOutput service.CreateRecommendationWorkflowOutput
	transcriptErr    error
	cvOutput         service.CreateRecommendationWorkflowOutput
	cvErr            error
	detailOutput     repository.SubmissionDetail
	detailErr        error
	previewOutput    []dto.RecommendationAllowedCandidateInput
	previewErr       error
}

func (f *fakeRecommendationService) UploadDocument(ctx context.Context, input service.UploadDocumentInput) (service.UploadDocumentOutput, error) {
	return f.uploadOutput, f.uploadErr
}

func (f *fakeRecommendationService) CreateSubmission(ctx context.Context, input service.CreateSubmissionInput) (service.CreateSubmissionOutput, error) {
	return f.createOutput, f.createErr
}

func (f *fakeRecommendationService) CreateProfileRecommendation(ctx context.Context, userID string, req dto.CreateProfileRecommendationRequest) (service.CreateRecommendationWorkflowOutput, error) {
	if f.profileErr != nil {
		return service.CreateRecommendationWorkflowOutput{}, f.profileErr
	}
	if f.profileOutput.SubmissionID != "" {
		return f.profileOutput, nil
	}
	return service.CreateRecommendationWorkflowOutput{
		SubmissionID: "sub-profile",
		Status:       model.RecommendationStatusCompleted,
		ResultSetID:  "set-profile",
		Result: &dto.GlobalMatchAIRecommendationResponse{
			TopRecommendations: []dto.GlobalMatchAITopRecommendationResponse{{Rank: 1, UniversityName: "University A"}},
		},
	}, nil
}

func (f *fakeRecommendationService) CreateTranscriptRecommendation(ctx context.Context, userID string, req dto.CreateTranscriptRecommendationRequest) (service.CreateRecommendationWorkflowOutput, error) {
	if f.transcriptErr != nil {
		return service.CreateRecommendationWorkflowOutput{}, f.transcriptErr
	}
	if f.transcriptOutput.SubmissionID != "" {
		return f.transcriptOutput, nil
	}
	return service.CreateRecommendationWorkflowOutput{
		SubmissionID: "sub-transcript",
		Status:       model.RecommendationStatusCompleted,
		ResultSetID:  "set-transcript",
		Result: &dto.GlobalMatchAIRecommendationResponse{
			TopRecommendations: []dto.GlobalMatchAITopRecommendationResponse{{Rank: 1, UniversityName: "University B"}},
		},
	}, nil
}

func (f *fakeRecommendationService) CreateCVRecommendation(ctx context.Context, userID string, req dto.CreateCVRecommendationRequest) (service.CreateRecommendationWorkflowOutput, error) {
	if f.cvErr != nil {
		return service.CreateRecommendationWorkflowOutput{}, f.cvErr
	}
	if f.cvOutput.SubmissionID != "" {
		return f.cvOutput, nil
	}
	return service.CreateRecommendationWorkflowOutput{
		SubmissionID: "sub-cv",
		Status:       model.RecommendationStatusCompleted,
		ResultSetID:  "set-cv",
		Result: &dto.GlobalMatchAIRecommendationResponse{
			TopRecommendations: []dto.GlobalMatchAITopRecommendationResponse{{Rank: 1, UniversityName: "University C"}},
		},
	}, nil
}

func (f *fakeRecommendationService) GetSubmissionDetail(ctx context.Context, userID, submissionID string) (repository.SubmissionDetail, error) {
	return f.detailOutput, f.detailErr
}

func (f *fakeRecommendationService) PreviewAllowedCandidates(ctx context.Context, userID string, preferences dto.RecommendationPreferenceInput) ([]dto.RecommendationAllowedCandidateInput, error) {
	return f.previewOutput, f.previewErr
}

func setupRecommendationRouter(svc *fakeRecommendationService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	ctrl := controller.NewRecommendationController(svc)
	router := gin.New()

	router.POST("/recommendations/submissions", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.CreateSubmission)
	router.POST("/recommendations/profile", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.CreateProfileRecommendation)
	router.POST("/recommendations/transcript", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.CreateTranscriptRecommendation)
	router.POST("/recommendations/cv", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.CreateCVRecommendation)
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

func setupRecommendationRouterWithoutAuth(svc *fakeRecommendationService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	ctrl := controller.NewRecommendationController(svc)
	router := gin.New()
	router.POST("/recommendations/profile", ctrl.CreateProfileRecommendation)
	router.POST("/recommendations/transcript", ctrl.CreateTranscriptRecommendation)
	router.POST("/recommendations/cv", ctrl.CreateCVRecommendation)
	router.POST("/recommendations/documents", ctrl.UploadDocument)
	router.POST("/recommendations/submissions", ctrl.CreateSubmission)
	router.GET("/recommendations/submissions/:id", ctrl.GetSubmissionDetail)
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

func TestUploadRecommendationDocumentServiceErrorController(t *testing.T) {
	svc := &fakeRecommendationService{uploadErr: errs.ErrInvalidInput}
	router := setupRecommendationRouter(svc)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("document_type", "transcript")
	fw, _ := writer.CreateFormFile("file", "transcript.pdf")
	_, _ = fw.Write([]byte("dummy"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/recommendations/documents", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestCreateRecommendationSubmissionInvalidInputController(t *testing.T) {
	svc := &fakeRecommendationService{createErr: errs.ErrNoDocumentProvided}
	router := setupRecommendationRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/recommendations/submissions", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestCreateRecommendationSubmissionDocumentNotFoundController(t *testing.T) {
	svc := &fakeRecommendationService{createErr: errs.ErrDocumentNotFound}
	router := setupRecommendationRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/recommendations/submissions", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d got %d", http.StatusNotFound, rec.Code)
	}
}

func TestCreateRecommendationSubmissionUnauthorizedController(t *testing.T) {
	svc := &fakeRecommendationService{}
	router := setupRecommendationRouterWithoutAuth(svc)

	req := httptest.NewRequest(http.MethodPost, "/recommendations/submissions", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestCreateProfileRecommendationInvalidInputController(t *testing.T) {
	svc := &fakeRecommendationService{}
	router := setupRecommendationRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/recommendations/profile", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "multipart/form-data")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestCreateProfileRecommendationSuccessController(t *testing.T) {
	svc := &fakeRecommendationService{}
	router := setupRecommendationRouter(svc)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("countries", "Japan")
	_ = writer.WriteField("degree_level", "Master")
	fw1, _ := writer.CreateFormFile("transcript_file", "transcript.pdf")
	_, _ = fw1.Write([]byte("dummy transcript"))
	fw2, _ := writer.CreateFormFile("cv_file", "cv.pdf")
	_, _ = fw2.Write([]byte("dummy cv"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/recommendations/profile", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}
}

func TestCreateCVRecommendationSuccessController(t *testing.T) {
	svc := &fakeRecommendationService{}
	router := setupRecommendationRouter(svc)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("countries", "Japan")
	fw, _ := writer.CreateFormFile("cv_file", "cv.pdf")
	_, _ = fw.Write([]byte("dummy cv"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/recommendations/cv", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}
}

func TestCreateTranscriptRecommendationSuccessController(t *testing.T) {
	svc := &fakeRecommendationService{}
	router := setupRecommendationRouter(svc)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("countries", "Japan")
	fw, _ := writer.CreateFormFile("transcript_file", "transcript.pdf")
	_, _ = fw.Write([]byte("dummy transcript"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/recommendations/transcript", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}
}

func TestCreateTranscriptRecommendationInvalidInputController(t *testing.T) {
	svc := &fakeRecommendationService{}
	router := setupRecommendationRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/recommendations/transcript", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "multipart/form-data")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestCreateCVRecommendationInvalidInputController(t *testing.T) {
	svc := &fakeRecommendationService{}
	router := setupRecommendationRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/recommendations/cv", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "multipart/form-data")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestCreateTranscriptRecommendationExternalServiceErrorController(t *testing.T) {
	svc := &fakeRecommendationService{transcriptErr: errs.ErrExternalService}
	router := setupRecommendationRouter(svc)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw, _ := writer.CreateFormFile("transcript_file", "transcript.pdf")
	_, _ = fw.Write([]byte("dummy transcript"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/recommendations/transcript", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected %d got %d body=%s", http.StatusBadGateway, rec.Code, rec.Body.String())
	}
}

func TestCreateCVRecommendationExternalServiceErrorController(t *testing.T) {
	svc := &fakeRecommendationService{cvErr: errs.ErrExternalService}
	router := setupRecommendationRouter(svc)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw, _ := writer.CreateFormFile("cv_file", "cv.pdf")
	_, _ = fw.Write([]byte("dummy cv"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/recommendations/cv", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected %d got %d body=%s", http.StatusBadGateway, rec.Code, rec.Body.String())
	}
}

func TestCreateProfileRecommendationInternalServerErrorController(t *testing.T) {
	svc := &fakeRecommendationService{profileErr: errors.New("boom")}
	router := setupRecommendationRouter(svc)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw1, _ := writer.CreateFormFile("transcript_file", "transcript.pdf")
	_, _ = fw1.Write([]byte("dummy transcript"))
	fw2, _ := writer.CreateFormFile("cv_file", "cv.pdf")
	_, _ = fw2.Write([]byte("dummy cv"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/recommendations/profile", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d got %d body=%s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}
}

func TestGetRecommendationSubmissionInvalidIDController(t *testing.T) {
	svc := &fakeRecommendationService{}
	router := setupRecommendationRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/recommendations/submissions/invalid-id", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetRecommendationSubmissionInternalServerErrorController(t *testing.T) {
	svc := &fakeRecommendationService{detailErr: errors.New("boom")}
	router := setupRecommendationRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/recommendations/submissions/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestGetRecommendationSubmissionUnauthorizedController(t *testing.T) {
	svc := &fakeRecommendationService{}
	router := setupRecommendationRouterWithoutAuth(svc)

	req := httptest.NewRequest(http.MethodGet, "/recommendations/submissions/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d got %d", http.StatusUnauthorized, rec.Code)
	}
}
