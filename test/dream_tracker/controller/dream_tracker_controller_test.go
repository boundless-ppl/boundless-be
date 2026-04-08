package controller_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type fakeDreamTrackerService struct {
	createOutput service.CreateDreamTrackerOutput
	createErr    error
	listOutput   []repository.DreamTrackerDetail
	listErr      error
	groupedOutput service.GroupedDreamTrackersOutput
	groupedErr    error
	dashboard    service.DreamTrackerDashboardSummary
	dashboardErr error
	detailOutput repository.DreamTrackerDetail
	detailErr    error
	document     model.Document
	documentErr  error
	uploadOutput service.UploadDreamRequirementDocumentOutput
	uploadStatus int
	uploadErr    error
	submitOutput service.SubmitDreamRequirementOutput
	submitErr    error
}

func (f *fakeDreamTrackerService) CreateDreamTracker(ctx context.Context, input service.CreateDreamTrackerInput) (service.CreateDreamTrackerOutput, error) {
	return f.createOutput, f.createErr
}

func (f *fakeDreamTrackerService) ListDreamTrackers(ctx context.Context, userID string) ([]repository.DreamTrackerDetail, error) {
	return f.listOutput, f.listErr
}

func (f *fakeDreamTrackerService) GetGroupedDreamTrackers(ctx context.Context, userID string, selectedDreamTrackerID *string, includeDefaultDetail bool) (service.GroupedDreamTrackersOutput, error) {
	return f.groupedOutput, f.groupedErr
}

func (f *fakeDreamTrackerService) GetDreamTrackerDashboardSummary(ctx context.Context, userID string) (service.DreamTrackerDashboardSummary, error) {
	return f.dashboard, f.dashboardErr
}

func (f *fakeDreamTrackerService) GetDreamTrackerDetail(ctx context.Context, userID, dreamTrackerID string) (repository.DreamTrackerDetail, error) {
	return f.detailOutput, f.detailErr
}

func (f *fakeDreamTrackerService) GetDocumentDetail(ctx context.Context, userID, documentID string) (model.Document, error) {
	return f.document, f.documentErr
}

func (f *fakeDreamTrackerService) UploadDreamRequirementDocument(ctx context.Context, input service.UploadDreamRequirementDocumentInput) (service.UploadDreamRequirementDocumentOutput, int, error) {
	return f.uploadOutput, f.uploadStatus, f.uploadErr
}

func (f *fakeDreamTrackerService) SubmitDreamRequirement(ctx context.Context, input service.SubmitDreamRequirementInput) (service.SubmitDreamRequirementOutput, error) {
	return f.submitOutput, f.submitErr
}

func setupDreamTrackerRouter(svc *fakeDreamTrackerService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	ctrl := controller.NewDreamTrackerController(svc)
	router := gin.New()

	router.POST("/dream-trackers", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.CreateDreamTracker)
	router.GET("/dream-trackers", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.ListDreamTrackers)
	router.GET("/dream-trackers/summary", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.GetDreamTrackerDashboardSummary)
	router.GET("/dream-trackers/grouped", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.GetGroupedDreamTrackers)
	router.GET("/dream-trackers/:id", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.GetDreamTrackerDetail)
	router.GET("/dream-trackers/documents/:id", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.GetDocumentDetail)
	router.POST("/dream-trackers/requirements/:id/submit", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.SubmitDreamRequirement)
	router.POST("/dream-trackers/requirements/:id/document", func(ctx *gin.Context) {
		ctx.Set(middleware.UserIDContextKey, "user-1")
		ctx.Next()
	}, ctrl.UploadDreamRequirementDocument)

	return router
}

func TestCreateDreamTrackerSuccessController(t *testing.T) {
	trackerID := uuid.NewString()
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{
		createOutput: service.CreateDreamTrackerOutput{
			DreamTracker: model.DreamTracker{
				DreamTrackerID: trackerID,
				Status:         model.DreamTrackerStatusActive,
			},
		},
	})

	body := []byte(`{"program_id":"program-1","title":"Target A","source_type":"MANUAL"}`)
	req := httptest.NewRequest(http.MethodPost, "/dream-trackers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d got %d", http.StatusCreated, rec.Code)
	}
}

func TestCreateDreamTrackerUnauthorizedController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := controller.NewDreamTrackerController(&fakeDreamTrackerService{})
	router := gin.New()
	router.POST("/dream-trackers", ctrl.CreateDreamTracker)

	req := httptest.NewRequest(http.MethodPost, "/dream-trackers", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestCreateDreamTrackerInvalidInputController(t *testing.T) {
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{})

	req := httptest.NewRequest(http.MethodPost, "/dream-trackers", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestCreateDreamTrackerServiceInvalidInputController(t *testing.T) {
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{createErr: errs.ErrInvalidInput})
	req := httptest.NewRequest(http.MethodPost, "/dream-trackers", bytes.NewBufferString(`{"program_id":"program-1","title":"Target A","source_type":"MANUAL"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestCreateDreamTrackerInternalErrorController(t *testing.T) {
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{createErr: errors.New("boom")})
	req := httptest.NewRequest(http.MethodPost, "/dream-trackers", bytes.NewBufferString(`{"program_id":"program-1","title":"Target A","source_type":"MANUAL"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestGetDreamTrackerNotFoundController(t *testing.T) {
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{detailErr: errs.ErrDreamTrackerNotFound})

	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d got %d", http.StatusNotFound, rec.Code)
	}
}

func TestGetDreamTrackerSuccessController(t *testing.T) {
	now := time.Now().UTC()
	rawMessages := "plain error"
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{
		detailOutput: repository.DreamTrackerDetail{
			DreamTracker: model.DreamTracker{
				DreamTrackerID: "d669bc06-d6e2-4592-a1a3-e6c64d846b97",
				UserID:         "user-1",
				ProgramID:      "program-1",
				Title:          "Plan A",
				Status:         model.DreamTrackerStatusActive,
				CreatedAt:      now,
				UpdatedAt:      now,
				SourceType:     "MANUAL",
			},
			Summary: model.DreamTrackerSummary{
				CompletionPercentage:  100,
				CompletedRequirements: 1,
				TotalRequirements:     1,
			},
			ProgramInfo: model.DreamTrackerProgramInfo{
				ProgramID: "program-1",
			},
			Requirements: []model.DreamRequirementDetail{
				{
					DreamRequirementStatus: model.DreamRequirementStatus{
						DreamReqStatusID: "req-status-1",
						ReqCatalogID:     "req-1",
						Status:           model.DreamRequirementStatusUploaded,
						AIMessages:       &rawMessages,
						CreatedAt:        now,
					},
					RequirementLabel: "Transcript",
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/d669bc06-d6e2-4592-a1a3-e6c64d846b97", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		StatusLabel string `json:"status_label"`
		Progress    struct {
			Percentage int `json:"percentage"`
		} `json:"progress"`
		Summary struct {
			CompletionPercentage int `json:"completion_percentage"`
		} `json:"summary"`
		Requirements []struct {
			RequirementLabel string  `json:"requirement_label"`
			Label            string  `json:"label"`
			StatusLabel      string  `json:"status_label"`
			StatusVariant    string  `json:"status_variant"`
			Message          *string `json:"message"`
		} `json:"requirements"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload.Summary.CompletionPercentage != 100 {
		t.Fatalf("unexpected summary: %+v", payload.Summary)
	}
	if payload.StatusLabel != "Sedang Diproses" || payload.Progress.Percentage != 100 {
		t.Fatalf("unexpected tracker presentation payload: %+v", payload)
	}
	if len(payload.Requirements) != 1 || payload.Requirements[0].RequirementLabel != "Transcript" || payload.Requirements[0].Label != "Transcript" {
		t.Fatalf("unexpected requirements payload: %+v", payload.Requirements)
	}
	if payload.Requirements[0].StatusLabel != "Berhasil diunggah" || payload.Requirements[0].StatusVariant != "SUCCESS" {
		t.Fatalf("unexpected requirement presentation payload: %+v", payload.Requirements[0])
	}
}

func TestListDreamTrackersSuccessController(t *testing.T) {
	now := time.Now().UTC()
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{
		listOutput: []repository.DreamTrackerDetail{
			{
				DreamTracker: model.DreamTracker{
					DreamTrackerID: "d669bc06-d6e2-4592-a1a3-e6c64d846b97",
					UserID:         "user-1",
					ProgramID:      "program-1",
					Title:          "Plan A",
					Status:         model.DreamTrackerStatusActive,
					CreatedAt:      now,
					UpdatedAt:      now,
					SourceType:     "MANUAL",
				},
				Summary: model.DreamTrackerSummary{
					CompletionPercentage: 75,
					TotalRequirements:    4,
				},
				ProgramInfo: model.DreamTrackerProgramInfo{ProgramID: "program-1"},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/dream-trackers", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Items []struct {
			DreamTrackerID string `json:"dream_tracker_id"`
			StatusLabel    string `json:"status_label"`
			StatusVariant  string `json:"status_variant"`
			Progress       struct {
				Percentage int `json:"percentage"`
			} `json:"progress"`
			Summary struct {
				CompletionPercentage int `json:"completion_percentage"`
			} `json:"summary"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].Summary.CompletionPercentage != 75 {
		t.Fatalf("unexpected list payload: %+v", payload.Items)
	}
	if payload.Items[0].StatusLabel != "Sedang Diproses" || payload.Items[0].StatusVariant != "IN_PROGRESS" || payload.Items[0].Progress.Percentage != 75 {
		t.Fatalf("unexpected list presentation payload: %+v", payload.Items[0])
	}
}

func TestGetDreamTrackerDashboardSummarySuccessController(t *testing.T) {
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{
		dashboard: service.DreamTrackerDashboardSummary{
			TotalTrackers:        3,
			IncompleteTrackers:   2,
			CompletedTrackers:    1,
			NearDeadlineTrackers: 1,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/summary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		TotalApplications int `json:"total_applications"`
		IncompleteCount   int `json:"incomplete_count"`
		CompletedCount    int `json:"completed_count"`
		DeadlineNearCount int `json:"deadline_near_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload.TotalApplications != 3 || payload.CompletedCount != 1 || payload.DeadlineNearCount != 1 {
		t.Fatalf("unexpected dashboard payload: %+v", payload)
	}
}

func TestGetGroupedDreamTrackersSuccessController(t *testing.T) {
	now := time.Now().UTC()
	defaultID := "tracker-1"
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{
		groupedOutput: service.GroupedDreamTrackersOutput{
			DefaultSelectedDreamTrackerID: &defaultID,
			Universities: []service.DreamTrackerUniversityGroup{{
				UniversityName: "University of Bristol",
				Items: []service.DreamTrackerGroupItem{{
					DreamTrackerID:       "tracker-1",
					Title:                "University of Bristol",
					Status:               model.DreamTrackerStatusActive,
					StatusLabel:          "Sedang Diproses",
					CompletionPercentage: 33,
					IsSelected:           true,
				}},
			}},
			DefaultDetail: &repository.DreamTrackerDetail{
				DreamTracker: model.DreamTracker{
					DreamTrackerID: "tracker-1",
					UserID:         "user-1",
					ProgramID:      "program-1",
					Title:          "University of Bristol",
					Status:         model.DreamTrackerStatusActive,
					CreatedAt:      now,
					UpdatedAt:      now,
				},
				ProgramInfo: model.DreamTrackerProgramInfo{ProgramID: "program-1"},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/grouped?include_default_detail=true", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		DefaultSelectedDreamTrackerID string `json:"default_selected_dream_tracker_id"`
		Universities                  []struct {
			UniversityName string `json:"university_name"`
		} `json:"universities"`
		DefaultDetail *struct {
			DreamTrackerID string `json:"dream_tracker_id"`
		} `json:"default_detail"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload.DefaultSelectedDreamTrackerID != "tracker-1" || len(payload.Universities) != 1 || payload.DefaultDetail == nil {
		t.Fatalf("unexpected grouped payload: %+v", payload)
	}
}

func TestGetDreamTrackerInvalidIDController(t *testing.T) {
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{})
	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/invalid-id", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetDreamTrackerInternalErrorController(t *testing.T) {
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{detailErr: errors.New("boom")})
	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestGetDocumentSuccessController(t *testing.T) {
	now := time.Now().UTC()
	docID := uuid.NewString()
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{
		document: model.Document{
			DocumentID: docID,
			UserID:     "user-1",
			Nama:       "Transcript",
			MIMEType:   "application/pdf",
			UploadedAt: now,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/documents/"+docID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, rec.Code)
	}

	var payload struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload.DocumentID != docID {
		t.Fatalf("expected %s got %s", docID, payload.DocumentID)
	}
}

func TestGetDocumentNotFoundController(t *testing.T) {
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{documentErr: errs.ErrDocumentNotFound})
	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/documents/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d got %d", http.StatusNotFound, rec.Code)
	}
}

func TestGetDocumentInvalidIDController(t *testing.T) {
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{})
	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/documents/invalid-id", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetDocumentInternalErrorController(t *testing.T) {
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{documentErr: errors.New("boom")})
	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/documents/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestSubmitDreamRequirementSuccessController(t *testing.T) {
	reqStatusID := uuid.NewString()
	docID := uuid.NewString()
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{
		submitOutput: service.SubmitDreamRequirementOutput{
			Requirement: model.DreamRequirementStatus{
				DreamReqStatusID: reqStatusID,
				DocumentID:       &docID,
				Status:           model.DreamRequirementStatusVerified,
			},
			AIMessages: []string{"valid document"},
		},
	})

	body := []byte(`{"document_id":"` + docID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/dream-trackers/requirements/"+reqStatusID+"/submit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		StatusLabel   string   `json:"status_label"`
		StatusVariant string   `json:"status_variant"`
		AIMessages    []string `json:"ai_messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload.StatusLabel != "Berhasil diunggah" || payload.StatusVariant != "SUCCESS" || len(payload.AIMessages) != 1 {
		t.Fatalf("unexpected submit payload: %+v", payload)
	}
}

func TestUploadDreamRequirementDocumentSuccessController(t *testing.T) {
	reqStatusID := uuid.NewString()
	docID := uuid.NewString()
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{
		uploadStatus: http.StatusOK,
		uploadOutput: service.UploadDreamRequirementDocumentOutput{
			Requirement: model.DreamRequirementStatus{
				DreamReqStatusID: reqStatusID,
				DocumentID:       &docID,
				Status:           model.DreamRequirementStatusReused,
			},
			Document: &model.Document{
				DocumentID:       docID,
				DocumentType:     model.DocumentType("KTP"),
				OriginalFilename: "ktp.pdf",
				PublicURL:        "https://example.com/ktp.pdf",
				UploadedAt:       time.Now().UTC(),
			},
			Review: model.DreamRequirementReview{
				Source:            "REUSED_EXISTING",
				Status:            "SKIPPED",
				IsReused:          true,
				IsAlreadyVerified: true,
			},
		},
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("document_type", "KTP")
	_ = writer.WriteField("reuse_if_exists", "true")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/dream-trackers/requirements/"+reqStatusID+"/document", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Status   string `json:"status"`
		Document *struct {
			DocumentID string `json:"document_id"`
		} `json:"document"`
		Review struct {
			Status string `json:"status"`
		} `json:"review"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload.Status != "REUSED" || payload.Document == nil || payload.Document.DocumentID != docID || payload.Review.Status != "SKIPPED" {
		t.Fatalf("unexpected upload payload: %+v", payload)
	}
}

func TestSubmitDreamRequirementInvalidIDController(t *testing.T) {
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{})
	req := httptest.NewRequest(http.MethodPost, "/dream-trackers/requirements/invalid-id/submit", bytes.NewBufferString(`{"document_id":"abc"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSubmitDreamRequirementDocumentNotFoundController(t *testing.T) {
	reqStatusID := uuid.NewString()
	docID := uuid.NewString()
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{submitErr: errs.ErrDocumentNotFound})
	req := httptest.NewRequest(http.MethodPost, "/dream-trackers/requirements/"+reqStatusID+"/submit", bytes.NewBufferString(`{"document_id":"`+docID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d got %d", http.StatusNotFound, rec.Code)
	}
}

func TestSubmitDreamRequirementInvalidBodyController(t *testing.T) {
	reqStatusID := uuid.NewString()
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{})
	req := httptest.NewRequest(http.MethodPost, "/dream-trackers/requirements/"+reqStatusID+"/submit", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSubmitDreamRequirementRequirementNotFoundController(t *testing.T) {
	reqStatusID := uuid.NewString()
	docID := uuid.NewString()
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{submitErr: errs.ErrDreamRequirementNotFound})
	req := httptest.NewRequest(http.MethodPost, "/dream-trackers/requirements/"+reqStatusID+"/submit", bytes.NewBufferString(`{"document_id":"`+docID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d got %d", http.StatusNotFound, rec.Code)
	}
}

func TestSubmitDreamRequirementInternalErrorController(t *testing.T) {
	reqStatusID := uuid.NewString()
	docID := uuid.NewString()
	router := setupDreamTrackerRouter(&fakeDreamTrackerService{submitErr: errors.New("boom")})
	req := httptest.NewRequest(http.MethodPost, "/dream-trackers/requirements/"+reqStatusID+"/submit", bytes.NewBufferString(`{"document_id":"`+docID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d got %d", http.StatusInternalServerError, rec.Code)
	}
}
