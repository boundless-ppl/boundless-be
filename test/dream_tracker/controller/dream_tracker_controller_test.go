package controller_test

import (
	"bytes"
	"context"
	"encoding/json"
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
)

const (
	testUserID       = "user-1"
	testDreamReqID   = "11111111-1111-1111-1111-111111111111"
	testDocumentID   = "22222222-2222-2222-2222-222222222222"
	testDreamTrackID = "33333333-3333-3333-3333-333333333333"
)

type fakeDreamTrackerService struct {
	createOut   service.CreateDreamTrackerOutput
	createErr   error
	createInput service.CreateDreamTrackerInput

	listOut []repository.DreamTrackerDetail
	listErr error

	groupedOut   service.GroupedDreamTrackersOutput
	groupedErr   error
	groupedUser  string
	groupedSelID *string
	groupedIncl  bool

	summaryOut service.DreamTrackerDashboardSummary
	summaryErr error

	detailOut repository.DreamTrackerDetail
	detailErr error

	documentOut model.Document
	documentErr error

	uploadOut   service.UploadDreamRequirementDocumentOutput
	uploadCode  int
	uploadErr   error
	uploadInput service.UploadDreamRequirementDocumentInput

	submitOut   service.SubmitDreamRequirementOutput
	submitErr   error
	submitInput service.SubmitDreamRequirementInput
}

func (f *fakeDreamTrackerService) CreateDreamTracker(ctx context.Context, input service.CreateDreamTrackerInput) (service.CreateDreamTrackerOutput, error) {
	f.createInput = input
	if f.createErr != nil {
		return service.CreateDreamTrackerOutput{}, f.createErr
	}
	return f.createOut, nil
}

func (f *fakeDreamTrackerService) ListDreamTrackers(ctx context.Context, userID string) ([]repository.DreamTrackerDetail, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listOut, nil
}

func (f *fakeDreamTrackerService) GetGroupedDreamTrackers(ctx context.Context, userID string, selectedDreamTrackerID *string, includeDefaultDetail bool) (service.GroupedDreamTrackersOutput, error) {
	f.groupedUser = userID
	f.groupedSelID = selectedDreamTrackerID
	f.groupedIncl = includeDefaultDetail
	if f.groupedErr != nil {
		return service.GroupedDreamTrackersOutput{}, f.groupedErr
	}
	return f.groupedOut, nil
}

func (f *fakeDreamTrackerService) GetDreamTrackerDashboardSummary(ctx context.Context, userID string) (service.DreamTrackerDashboardSummary, error) {
	if f.summaryErr != nil {
		return service.DreamTrackerDashboardSummary{}, f.summaryErr
	}
	return f.summaryOut, nil
}

func (f *fakeDreamTrackerService) GetDreamTrackerDetail(ctx context.Context, userID, dreamTrackerID string) (repository.DreamTrackerDetail, error) {
	if f.detailErr != nil {
		return repository.DreamTrackerDetail{}, f.detailErr
	}
	return f.detailOut, nil
}

func (f *fakeDreamTrackerService) GetDocumentDetail(ctx context.Context, userID, documentID string) (model.Document, error) {
	if f.documentErr != nil {
		return model.Document{}, f.documentErr
	}
	return f.documentOut, nil
}

func (f *fakeDreamTrackerService) UploadDreamRequirementDocument(ctx context.Context, input service.UploadDreamRequirementDocumentInput) (service.UploadDreamRequirementDocumentOutput, int, error) {
	f.uploadInput = input
	if f.uploadErr != nil {
		return service.UploadDreamRequirementDocumentOutput{}, 0, f.uploadErr
	}
	return f.uploadOut, f.uploadCode, nil
}

func (f *fakeDreamTrackerService) SubmitDreamRequirement(ctx context.Context, input service.SubmitDreamRequirementInput) (service.SubmitDreamRequirementOutput, error) {
	f.submitInput = input
	if f.submitErr != nil {
		return service.SubmitDreamRequirementOutput{}, f.submitErr
	}
	return f.submitOut, nil
}

func withUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.UserIDContextKey, testUserID)
		c.Next()
	}
}

func TestCreateDreamTrackerUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := controller.NewDreamTrackerController(&fakeDreamTrackerService{})
	r := gin.New()
	r.POST("/dream-trackers", c.CreateDreamTracker)

	req := httptest.NewRequest(http.MethodPost, "/dream-trackers", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestCreateDreamTrackerSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeDreamTrackerService{
		createOut: service.CreateDreamTrackerOutput{
			DreamTracker: model.DreamTracker{
				DreamTrackerID: testDreamTrackID,
				Status:         model.DreamTrackerStatusActive,
			},
		},
	}
	c := controller.NewDreamTrackerController(svc)
	r := gin.New()
	r.POST("/dream-trackers", withUser(), c.CreateDreamTracker)

	body := `{"program_id":"program-1","source_type":"MANUAL","status":"ACTIVE","title":"My Dream","scholarship_name":"LPDP"}`
	req := httptest.NewRequest(http.MethodPost, "/dream-trackers", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, rec.Code)
	}
	if svc.createInput.UserID != testUserID || svc.createInput.ProgramID != "program-1" {
		t.Fatalf("unexpected create input: %+v", svc.createInput)
	}
}

func TestCreateDreamTrackerErrorMappings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name string
		err  error
		code int
	}{
		{"unauthorized", errs.ErrUnauthorized, http.StatusUnauthorized},
		{"invalid_input", errs.ErrInvalidInput, http.StatusBadRequest},
		{"internal", context.DeadlineExceeded, http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeDreamTrackerService{createErr: tc.err}
			c := controller.NewDreamTrackerController(svc)
			r := gin.New()
			r.POST("/dream-trackers", withUser(), c.CreateDreamTracker)

			body := `{"program_id":"program-1","source_type":"MANUAL","status":"ACTIVE","title":"My Dream"}`
			req := httptest.NewRequest(http.MethodPost, "/dream-trackers", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tc.code {
				t.Fatalf("expected %d, got %d", tc.code, rec.Code)
			}
		})
	}
}

func TestListDreamTrackersHandlesNotFoundError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeDreamTrackerService{listErr: errs.ErrDreamTrackerNotFound}
	c := controller.NewDreamTrackerController(svc)
	r := gin.New()
	r.GET("/dream-trackers", withUser(), c.ListDreamTrackers)

	req := httptest.NewRequest(http.MethodGet, "/dream-trackers", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestGetDreamTrackerDashboardSummarySuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeDreamTrackerService{
		summaryOut: service.DreamTrackerDashboardSummary{
			TotalTrackers:        4,
			IncompleteTrackers:   3,
			CompletedTrackers:    1,
			NearDeadlineTrackers: 2,
		},
	}
	c := controller.NewDreamTrackerController(svc)
	r := gin.New()
	r.GET("/dream-trackers/dashboard", withUser(), c.GetDreamTrackerDashboardSummary)

	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/dashboard", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestGetGroupedDreamTrackersInvalidQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := controller.NewDreamTrackerController(&fakeDreamTrackerService{})
	r := gin.New()
	r.GET("/dream-trackers/grouped", withUser(), c.GetGroupedDreamTrackers)

	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/grouped?include_default_detail=abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetGroupedDreamTrackersSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defaultID := testDreamTrackID
	svc := &fakeDreamTrackerService{
		groupedOut: service.GroupedDreamTrackersOutput{
			DefaultSelectedDreamTrackerID: &defaultID,
			Universities: []service.DreamTrackerUniversityGroup{{
				UniversityName: "University A",
				Items: []service.DreamTrackerGroupItem{{
					DreamTrackerID: testDreamTrackID,
					Title:          "My Dream",
					Status:         model.DreamTrackerStatusActive,
				}},
			}},
		},
	}
	c := controller.NewDreamTrackerController(svc)
	r := gin.New()
	r.GET("/dream-trackers/grouped", withUser(), c.GetGroupedDreamTrackers)

	url := "/dream-trackers/grouped?include_default_detail=true&selected_dream_tracker_id=" + testDreamTrackID
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if svc.groupedSelID == nil || *svc.groupedSelID != testDreamTrackID || !svc.groupedIncl {
		t.Fatalf("unexpected grouped input: selected=%v include=%v", svc.groupedSelID, svc.groupedIncl)
	}
}

func TestGetDocumentDetailErrorsAndSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name string
		err  error
		code int
	}{
		{"not_found", errs.ErrDocumentNotFound, http.StatusNotFound},
		{"invalid_input", errs.ErrInvalidInput, http.StatusBadRequest},
		{"success", nil, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeDreamTrackerService{
				documentErr: tc.err,
				documentOut: model.Document{
					DocumentID: testDocumentID,
					UserID:     testUserID,
					UploadedAt: time.Now().UTC(),
				},
			}
			c := controller.NewDreamTrackerController(svc)
			r := gin.New()
			r.GET("/documents/:id", withUser(), c.GetDocumentDetail)

			req := httptest.NewRequest(http.MethodGet, "/documents/"+testDocumentID, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tc.code {
				t.Fatalf("expected %d, got %d", tc.code, rec.Code)
			}
		})
	}
}

func TestUploadDreamRequirementDocumentValidationAndSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeDreamTrackerService{
		uploadCode: http.StatusOK,
		uploadOut: service.UploadDreamRequirementDocumentOutput{
			Requirement: model.DreamRequirementStatus{
				DreamReqStatusID: testDreamReqID,
				Status:           model.DreamRequirementStatusUploaded,
			},
		},
	}
	c := controller.NewDreamTrackerController(svc)
	r := gin.New()
	r.POST("/requirements/:id/upload", withUser(), c.UploadDreamRequirementDocument)

	reqBad := httptest.NewRequest(http.MethodPost, "/requirements/"+testDreamReqID+"/upload", bytes.NewBufferString("reuse_if_exists=x"))
	reqBad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recBad := httptest.NewRecorder()
	r.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, recBad.Code)
	}

	reqOK := httptest.NewRequest(http.MethodPost, "/requirements/"+testDreamReqID+"/upload", bytes.NewBufferString("reuse_if_exists=false&document_type=passport"))
	reqOK.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recOK := httptest.NewRecorder()
	r.ServeHTTP(recOK, reqOK)
	if recOK.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, recOK.Code)
	}
	if svc.uploadInput.DreamReqStatusID != testDreamReqID || svc.uploadInput.ReuseIfExists {
		t.Fatalf("unexpected upload input: %+v", svc.uploadInput)
	}
}

func TestUploadDreamRequirementDocumentErrorMappings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name string
		err  error
		code int
	}{
		{"unauthorized", errs.ErrUnauthorized, http.StatusUnauthorized},
		{"invalid_input", errs.ErrInvalidInput, http.StatusBadRequest},
		{"document_not_found", errs.ErrDocumentNotFound, http.StatusNotFound},
		{"requirement_not_found", errs.ErrDreamRequirementNotFound, http.StatusNotFound},
		{"internal", context.DeadlineExceeded, http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeDreamTrackerService{uploadErr: tc.err}
			c := controller.NewDreamTrackerController(svc)
			r := gin.New()
			r.POST("/requirements/:id/upload", withUser(), c.UploadDreamRequirementDocument)

			req := httptest.NewRequest(http.MethodPost, "/requirements/"+testDreamReqID+"/upload", bytes.NewBufferString("reuse_if_exists=false&document_type=passport"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tc.code {
				t.Fatalf("expected %d, got %d", tc.code, rec.Code)
			}
		})
	}
}

func TestSubmitDreamRequirementValidationAndMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeDreamTrackerService{submitErr: errs.ErrDreamRequirementNotFound}
	c := controller.NewDreamTrackerController(svc)
	r := gin.New()
	r.POST("/requirements/:id/submit", withUser(), c.SubmitDreamRequirement)

	reqBad := httptest.NewRequest(http.MethodPost, "/requirements/"+testDreamReqID+"/submit", bytes.NewBufferString(`{"document_id":"bad-id"}`))
	reqBad.Header.Set("Content-Type", "application/json")
	recBad := httptest.NewRecorder()
	r.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, recBad.Code)
	}

	body, _ := json.Marshal(dto.SubmitDreamRequirementRequest{DocumentID: testDocumentID})
	reqNotFound := httptest.NewRequest(http.MethodPost, "/requirements/"+testDreamReqID+"/submit", bytes.NewBuffer(body))
	reqNotFound.Header.Set("Content-Type", "application/json")
	recNotFound := httptest.NewRecorder()
	r.ServeHTTP(recNotFound, reqNotFound)
	if recNotFound.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, recNotFound.Code)
	}
}

func TestSubmitDreamRequirementSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	docID := testDocumentID
	svc := &fakeDreamTrackerService{
		submitOut: service.SubmitDreamRequirementOutput{
			Requirement: model.DreamRequirementStatus{
				DreamReqStatusID: testDreamReqID,
				DocumentID:       &docID,
				Status:           model.DreamRequirementStatusVerified,
			},
			AIMessages: []string{"ok"},
		},
	}
	c := controller.NewDreamTrackerController(svc)
	r := gin.New()
	r.POST("/requirements/:id/submit", withUser(), c.SubmitDreamRequirement)

	body, _ := json.Marshal(dto.SubmitDreamRequirementRequest{DocumentID: testDocumentID})
	req := httptest.NewRequest(http.MethodPost, "/requirements/"+testDreamReqID+"/submit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if svc.submitInput.DocumentID != testDocumentID {
		t.Fatalf("unexpected submit input: %+v", svc.submitInput)
	}
}

func TestGetDreamTrackerDetailInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := controller.NewDreamTrackerController(&fakeDreamTrackerService{})
	r := gin.New()
	r.GET("/dream-trackers/:id", withUser(), c.GetDreamTrackerDetail)

	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/not-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
