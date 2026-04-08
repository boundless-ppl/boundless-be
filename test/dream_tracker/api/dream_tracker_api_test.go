package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"boundless-be/api"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"
)

type fakeDreamAPIUserRepo struct{}

func (f *fakeDreamAPIUserRepo) Create(ctx context.Context, user model.User) (model.User, error) {
	return user, nil
}
func (f *fakeDreamAPIUserRepo) FindByEmail(ctx context.Context, email string) (model.User, error) {
	return model.User{}, repository.ErrUserNotFound
}
func (f *fakeDreamAPIUserRepo) FindByID(ctx context.Context, userID string) (model.User, error) {
	return model.User{}, repository.ErrUserNotFound
}
func (f *fakeDreamAPIUserRepo) Update(ctx context.Context, user model.User) error {
	return nil
}

type fakeDreamAPIRepo struct {
	tracker       repository.DreamTrackerDetail
	trackers      []model.DreamTracker
	details       map[string]repository.DreamTrackerDetail
	docs          map[string]model.Document
	reusableDoc   model.Document
	reusableFound bool
	reqs          map[string]model.DreamRequirementStatus
	seed          repository.DreamTrackerSeed
}

func (f *fakeDreamAPIRepo) CreateDreamTracker(ctx context.Context, tracker model.DreamTracker) (model.DreamTracker, error) {
	f.tracker.DreamTracker = tracker
	f.trackers = append(f.trackers, tracker)
	if tracker.FundingID != nil {
		f.tracker.Requirements = []model.DreamRequirementDetail{
			{
				DreamRequirementStatus: model.DreamRequirementStatus{
					DreamReqStatusID: "req-status-1",
					DreamTrackerID:   tracker.DreamTrackerID,
					ReqCatalogID:     "req-1",
					Status:           model.DreamRequirementStatusNotUploaded,
					CreatedAt:        tracker.CreatedAt,
				},
				RequirementLabel: "Transcript",
			},
		}
	}
	return tracker, nil
}

func (f *fakeDreamAPIRepo) FindDreamTrackersByUser(ctx context.Context, userID string) ([]model.DreamTracker, error) {
	items := make([]model.DreamTracker, 0, len(f.trackers))
	for _, tracker := range f.trackers {
		if tracker.UserID == userID {
			items = append(items, tracker)
		}
	}
	return items, nil
}

func (f *fakeDreamAPIRepo) ResolveDreamTrackerSeed(ctx context.Context, programID *string, sourceRecResultID *string) (repository.DreamTrackerSeed, error) {
	if f.seed.ProgramID != "" {
		return f.seed, nil
	}
	seed := repository.DreamTrackerSeed{ProgramID: "program-1", Title: "Target A"}
	if programID != nil && *programID != "" {
		seed.ProgramID = *programID
	}
	return seed, nil
}

func (f *fakeDreamAPIRepo) CreateDocument(ctx context.Context, doc model.Document) (model.Document, error) {
	if f.docs == nil {
		f.docs = map[string]model.Document{}
	}
	f.docs[doc.DocumentID] = doc
	return doc, nil
}

func (f *fakeDreamAPIRepo) FindDreamTrackerDetail(ctx context.Context, dreamTrackerID, userID string) (repository.DreamTrackerDetail, error) {
	if f.details != nil {
		detail, ok := f.details[dreamTrackerID]
		if !ok || detail.DreamTracker.UserID != userID {
			return repository.DreamTrackerDetail{}, errs.ErrDreamTrackerNotFound
		}
		return detail, nil
	}
	if f.tracker.DreamTracker.DreamTrackerID != dreamTrackerID || f.tracker.DreamTracker.UserID != userID {
		return repository.DreamTrackerDetail{}, errs.ErrDreamTrackerNotFound
	}
	return f.tracker, nil
}

func (f *fakeDreamAPIRepo) FindDocumentByIDAndUser(ctx context.Context, documentID, userID string) (model.Document, error) {
	doc, ok := f.docs[documentID]
	if !ok || doc.UserID != userID {
		return model.Document{}, errs.ErrDocumentNotFound
	}
	return doc, nil
}

func (f *fakeDreamAPIRepo) FindReusableDocumentByUserAndType(ctx context.Context, userID, documentType string) (model.Document, bool, error) {
	if !f.reusableFound {
		return model.Document{}, false, nil
	}
	return f.reusableDoc, true, nil
}

func (f *fakeDreamAPIRepo) FindDreamRequirementStatusByIDAndUser(ctx context.Context, dreamReqStatusID, userID string) (model.DreamRequirementStatus, error) {
	req, ok := f.reqs[dreamReqStatusID]
	if !ok {
		return model.DreamRequirementStatus{}, errs.ErrDreamRequirementNotFound
	}
	if f.tracker.DreamTracker.UserID != userID {
		return model.DreamRequirementStatus{}, errs.ErrDreamRequirementNotFound
	}
	return req, nil
}

func (f *fakeDreamAPIRepo) UpdateDreamRequirementStatus(ctx context.Context, requirement model.DreamRequirementStatus) error {
	if f.reqs == nil {
		f.reqs = map[string]model.DreamRequirementStatus{}
	}
	f.reqs[requirement.DreamReqStatusID] = requirement
	return nil
}

func setupDreamAPIHandler(t *testing.T, repo repository.DreamTrackerRepository) http.Handler {
	t.Helper()
	t.Setenv("AUTH_SECRET", "test-secret")
	return api.NewHandler(api.Dependencies{
		UserRepo:         &fakeDreamAPIUserRepo{},
		DreamTrackerRepo: repo,
	})
}

func issueDreamToken(t *testing.T) string {
	t.Helper()
	tm := service.NewHMACTokenManager("test-secret")
	tokens, err := tm.IssueTokens("user-1", "student")
	if err != nil {
		t.Fatal(err)
	}
	return tokens.AccessToken
}

func TestCreateAndGetDreamTrackerAPI(t *testing.T) {
	repo := &fakeDreamAPIRepo{}
	handler := setupDreamAPIHandler(t, repo)
	token := issueDreamToken(t)

	fundingID := "0b5030bb-11ca-4db4-a441-bf407fddd16d"
	body := []byte(`{"program_id":"program-1","funding_id":"` + fundingID + `","title":"Target A","source_type":"MANUAL"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/dream-trackers", bytes.NewBuffer(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected %d got %d body=%s", http.StatusCreated, createRec.Code, createRec.Body.String())
	}

	var createResp struct {
		DreamTrackerID string `json:"dream_tracker_id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("invalid create response: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/dream-trackers/"+createResp.DreamTrackerID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, getRec.Code, getRec.Body.String())
	}

	var getResp struct {
		DreamTrackerID string `json:"dream_tracker_id"`
		StatusLabel    string `json:"status_label"`
		Progress       struct {
			Percentage int `json:"percentage"`
		} `json:"progress"`
		Summary struct {
			TotalRequirements int `json:"total_requirements"`
		} `json:"summary"`
		Requirements []struct {
			Status           string `json:"status"`
			RequirementLabel string `json:"requirement_label"`
			Label            string `json:"label"`
			StatusLabel      string `json:"status_label"`
		} `json:"requirements"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("invalid get response: %v", err)
	}
	if getResp.Summary.TotalRequirements != 1 {
		t.Fatalf("unexpected summary payload: %+v", getResp.Summary)
	}
	if getResp.StatusLabel != "Sedang Diproses" || getResp.Progress.Percentage != 0 {
		t.Fatalf("unexpected presentation payload: %+v", getResp)
	}
	if len(getResp.Requirements) != 1 || getResp.Requirements[0].Status != "NOT_UPLOADED" || getResp.Requirements[0].RequirementLabel != "Transcript" || getResp.Requirements[0].Label != "Transcript" || getResp.Requirements[0].StatusLabel != "Belum diunggah" {
		t.Fatalf("unexpected requirements payload: %+v", getResp.Requirements)
	}
}

func TestGetDocumentAPI(t *testing.T) {
	docID := "9f0fdb35-6c31-4d48-a804-a89f6c29e3ef"
	repo := &fakeDreamAPIRepo{
		docs: map[string]model.Document{
			docID: {
				DocumentID: docID,
				UserID:     "user-1",
				Nama:       "Transcript",
				MIMEType:   "application/pdf",
				UploadedAt: time.Now().UTC(),
			},
		},
	}
	handler := setupDreamAPIHandler(t, repo)
	token := issueDreamToken(t)

	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/documents/"+docID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestListDreamTrackersAPI(t *testing.T) {
	now := time.Now().UTC()
	trackerID := "d669bc06-d6e2-4592-a1a3-e6c64d846b97"
	repo := &fakeDreamAPIRepo{
		trackers: []model.DreamTracker{
			{DreamTrackerID: trackerID, UserID: "user-1", ProgramID: "program-1", Title: "Target A", Status: model.DreamTrackerStatusActive, CreatedAt: now, UpdatedAt: now, SourceType: "MANUAL"},
		},
		details: map[string]repository.DreamTrackerDetail{
			trackerID: {
				DreamTracker: model.DreamTracker{DreamTrackerID: trackerID, UserID: "user-1", ProgramID: "program-1", Title: "Target A", Status: model.DreamTrackerStatusActive, CreatedAt: now, UpdatedAt: now, SourceType: "MANUAL"},
				Summary:      model.DreamTrackerSummary{CompletionPercentage: 75},
				ProgramInfo:  model.DreamTrackerProgramInfo{ProgramID: "program-1"},
			},
		},
	}
	handler := setupDreamAPIHandler(t, repo)
	token := issueDreamToken(t)

	req := httptest.NewRequest(http.MethodGet, "/dream-trackers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Items []struct {
			DreamTrackerID string `json:"dream_tracker_id"`
			StatusLabel    string `json:"status_label"`
			StatusVariant  string `json:"status_variant"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid list response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].DreamTrackerID != trackerID {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Items[0].StatusLabel != "Sedang Diproses" || payload.Items[0].StatusVariant != "IN_PROGRESS" {
		t.Fatalf("unexpected list presentation payload: %+v", payload.Items[0])
	}
}

func TestDreamTrackerSummaryAPI(t *testing.T) {
	now := time.Now().UTC()
	nearDeadline := now.Add(24 * time.Hour)
	repo := &fakeDreamAPIRepo{
		trackers: []model.DreamTracker{
			{DreamTrackerID: "tracker-1", UserID: "user-1"},
			{DreamTrackerID: "tracker-2", UserID: "user-1"},
		},
		details: map[string]repository.DreamTrackerDetail{
			"tracker-1": {
				DreamTracker: model.DreamTracker{DreamTrackerID: "tracker-1", UserID: "user-1", Status: model.DreamTrackerStatusActive},
				Requirements: []model.DreamRequirementDetail{
					{DreamRequirementStatus: model.DreamRequirementStatus{Status: model.DreamRequirementStatusUploaded}},
					{DreamRequirementStatus: model.DreamRequirementStatus{Status: model.DreamRequirementStatusNotUploaded}},
				},
				Milestones: []model.DreamKeyMilestone{{DeadlineDate: &nearDeadline}},
			},
			"tracker-2": {
				DreamTracker: model.DreamTracker{DreamTrackerID: "tracker-2", UserID: "user-1", Status: model.DreamTrackerStatusCompleted},
				Requirements: []model.DreamRequirementDetail{
					{DreamRequirementStatus: model.DreamRequirementStatus{Status: model.DreamRequirementStatusUploaded}},
				},
			},
		},
	}
	handler := setupDreamAPIHandler(t, repo)
	token := issueDreamToken(t)

	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

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
		t.Fatalf("invalid summary response: %v", err)
	}
	if payload.TotalApplications != 2 || payload.IncompleteCount != 1 || payload.CompletedCount != 1 || payload.DeadlineNearCount != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestGroupedDreamTrackersAPI(t *testing.T) {
	now := time.Now().UTC()
	trackerID := "d669bc06-d6e2-4592-a1a3-e6c64d846b97"
	repo := &fakeDreamAPIRepo{
		trackers: []model.DreamTracker{
			{DreamTrackerID: trackerID, UserID: "user-1", ProgramID: "program-1", Title: "University of Bristol", Status: model.DreamTrackerStatusActive, CreatedAt: now, UpdatedAt: now, SourceType: "MANUAL"},
		},
		details: map[string]repository.DreamTrackerDetail{
			trackerID: {
				DreamTracker: model.DreamTracker{DreamTrackerID: trackerID, UserID: "user-1", ProgramID: "program-1", Title: "University of Bristol", Status: model.DreamTrackerStatusActive, CreatedAt: now, UpdatedAt: now, SourceType: "MANUAL"},
				Summary:      model.DreamTrackerSummary{CompletionPercentage: 33},
				ProgramInfo: model.DreamTrackerProgramInfo{
					ProgramID:      "program-1",
					ProgramName:    stringPtr("MSc Computer Science"),
					UniversityName: stringPtr("University of Bristol"),
				},
				Fundings: []model.DreamTrackerFundingOption{{
					FundingID:    "funding-1",
					NamaBeasiswa: "LPDP Scholarship",
				}},
			},
		},
	}
	handler := setupDreamAPIHandler(t, repo)
	token := issueDreamToken(t)

	req := httptest.NewRequest(http.MethodGet, "/dream-trackers/grouped?include_default_detail=true", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		DefaultSelectedDreamTrackerID string `json:"default_selected_dream_tracker_id"`
		Universities                  []struct {
			UniversityName string `json:"university_name"`
		} `json:"universities"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid grouped response: %v", err)
	}
	if payload.DefaultSelectedDreamTrackerID != trackerID || len(payload.Universities) != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestSubmitDreamRequirementAPI(t *testing.T) {
	docID := "9f0fdb35-6c31-4d48-a804-a89f6c29e3ef"
	reqID := "d669bc06-d6e2-4592-a1a3-e6c64d846b97"
	repo := &fakeDreamAPIRepo{
		tracker: repository.DreamTrackerDetail{
			DreamTracker: model.DreamTracker{
				DreamTrackerID: "tracker-1",
				UserID:         "user-1",
			},
		},
		docs: map[string]model.Document{
			docID: {
				DocumentID: docID,
				UserID:     "user-1",
				PublicURL:  "https://example.com/doc.pdf",
				MIMEType:   "application/pdf",
				UploadedAt: time.Now().UTC(),
			},
		},
		reqs: map[string]model.DreamRequirementStatus{
			reqID: {
				DreamReqStatusID: reqID,
				DreamTrackerID:   "tracker-1",
				ReqCatalogID:     "req-1",
				Status:           model.DreamRequirementStatusNotUploaded,
			},
		},
	}
	handler := setupDreamAPIHandler(t, repo)
	token := issueDreamToken(t)

	req := httptest.NewRequest(http.MethodPost, "/dream-trackers/requirements/"+reqID+"/submit", bytes.NewBufferString(`{"document_id":"`+docID+`"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestSubmitDreamRequirementRejectsNonPDFAPI(t *testing.T) {
	docID := "9f0fdb35-6c31-4d48-a804-a89f6c29e3ef"
	reqID := "d669bc06-d6e2-4592-a1a3-e6c64d846b97"
	repo := &fakeDreamAPIRepo{
		tracker: repository.DreamTrackerDetail{
			DreamTracker: model.DreamTracker{
				DreamTrackerID: "tracker-1",
				UserID:         "user-1",
			},
		},
		docs: map[string]model.Document{
			docID: {
				DocumentID:       docID,
				UserID:           "user-1",
				OriginalFilename: "doc.png",
				MIMEType:         "image/png",
				UploadedAt:       time.Now().UTC(),
			},
		},
		reqs: map[string]model.DreamRequirementStatus{
			reqID: {
				DreamReqStatusID: reqID,
				DreamTrackerID:   "tracker-1",
				ReqCatalogID:     "req-1",
				Status:           model.DreamRequirementStatusNotUploaded,
			},
		},
	}
	handler := setupDreamAPIHandler(t, repo)
	token := issueDreamToken(t)

	req := httptest.NewRequest(http.MethodPost, "/dream-trackers/requirements/"+reqID+"/submit", bytes.NewBufferString(`{"document_id":"`+docID+`"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestUploadDreamRequirementDocumentRequiresFileAPI(t *testing.T) {
	reqID := "d669bc06-d6e2-4592-a1a3-e6c64d846b97"
	repo := &fakeDreamAPIRepo{
		tracker: repository.DreamTrackerDetail{
			DreamTracker: model.DreamTracker{
				DreamTrackerID: "tracker-1",
				UserID:         "user-1",
			},
		},
		reqs: map[string]model.DreamRequirementStatus{
			reqID: {
				DreamReqStatusID: reqID,
				DreamTrackerID:   "tracker-1",
				ReqCatalogID:     "req-1",
				Status:           model.DreamRequirementStatusNotUploaded,
			},
		},
	}
	handler := setupDreamAPIHandler(t, repo)
	token := issueDreamToken(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("document_type", "KTP")
	_ = writer.WriteField("reuse_if_exists", "true")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/dream-trackers/requirements/"+reqID+"/document", &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d body=%s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func stringPtr(value string) *string {
	return &value
}
