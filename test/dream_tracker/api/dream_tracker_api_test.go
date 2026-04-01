package api_test

import (
	"bytes"
	"context"
	"encoding/json"
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
	tracker repository.DreamTrackerDetail
	docs    map[string]model.Document
	reqs    map[string]model.DreamRequirementStatus
}

func (f *fakeDreamAPIRepo) CreateDreamTracker(ctx context.Context, tracker model.DreamTracker) (model.DreamTracker, error) {
	f.tracker.DreamTracker = tracker
	if tracker.FundingID != nil {
		f.tracker.Requirements = []model.DreamRequirementStatus{
			{
				DreamReqStatusID: "req-status-1",
				DreamTrackerID:   tracker.DreamTrackerID,
				ReqCatalogID:     "req-1",
				Status:           model.DreamRequirementStatusNotUploaded,
				CreatedAt:        tracker.CreatedAt,
			},
		}
	}
	return tracker, nil
}

func (f *fakeDreamAPIRepo) FindDreamTrackerDetail(ctx context.Context, dreamTrackerID, userID string) (repository.DreamTrackerDetail, error) {
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
		Requirements   []struct {
			Status string `json:"status"`
		} `json:"requirements"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("invalid get response: %v", err)
	}
	if len(getResp.Requirements) != 1 || getResp.Requirements[0].Status != "NOT_UPLOADED" {
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
