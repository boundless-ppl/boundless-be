package service_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"
)

type fakeDreamTrackerRepo struct {
	createdTracker model.DreamTracker
	createErr      error
	detail         repository.DreamTrackerDetail
	detailErr      error
	document       model.Document
	documentErr    error
	requirement    model.DreamRequirementStatus
	requirementErr error
	updated        model.DreamRequirementStatus
	updateErr      error
}

func (f *fakeDreamTrackerRepo) CreateDreamTracker(ctx context.Context, tracker model.DreamTracker) (model.DreamTracker, error) {
	f.createdTracker = tracker
	if f.createErr != nil {
		return model.DreamTracker{}, f.createErr
	}
	return tracker, nil
}

func (f *fakeDreamTrackerRepo) FindDreamTrackerDetail(ctx context.Context, dreamTrackerID, userID string) (repository.DreamTrackerDetail, error) {
	return f.detail, f.detailErr
}

func (f *fakeDreamTrackerRepo) FindDocumentByIDAndUser(ctx context.Context, documentID, userID string) (model.Document, error) {
	return f.document, f.documentErr
}

func (f *fakeDreamTrackerRepo) FindDreamRequirementStatusByIDAndUser(ctx context.Context, dreamReqStatusID, userID string) (model.DreamRequirementStatus, error) {
	return f.requirement, f.requirementErr
}

func (f *fakeDreamTrackerRepo) UpdateDreamRequirementStatus(ctx context.Context, requirement model.DreamRequirementStatus) error {
	f.updated = requirement
	return f.updateErr
}

type fakeDreamTrackerAIClient struct {
	reviewResponse dto.DreamRequirementReviewResponse
	reviewErr      error
	lastRequest    dto.DreamRequirementReviewRequest
}

func (f *fakeDreamTrackerAIClient) ReviewRequirement(ctx context.Context, req dto.DreamRequirementReviewRequest) (dto.DreamRequirementReviewResponse, error) {
	f.lastRequest = req
	return f.reviewResponse, f.reviewErr
}

func TestCreateDreamTrackerDefaultsStatusService(t *testing.T) {
	repo := &fakeDreamTrackerRepo{}
	svc := service.NewDreamTrackerServiceWithDeps(repo, nil)

	output, err := svc.CreateDreamTracker(context.Background(), service.CreateDreamTrackerInput{
		UserID:     "user-1",
		ProgramID:  "program-1",
		Title:      "Target A",
		SourceType: "MANUAL",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if output.DreamTracker.Status != model.DreamTrackerStatusActive {
		t.Fatalf("expected ACTIVE, got %q", output.DreamTracker.Status)
	}
	if repo.createdTracker.CreatedAt.IsZero() || repo.createdTracker.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps to be set")
	}
}

func TestGetDocumentDetailInvalidInputService(t *testing.T) {
	svc := service.NewDreamTrackerServiceWithDeps(&fakeDreamTrackerRepo{}, nil)

	_, err := svc.GetDocumentDetail(context.Background(), "", "")
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestGetDreamTrackerDetailService(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeDreamTrackerRepo{
		detail: repository.DreamTrackerDetail{
			DreamTracker: model.DreamTracker{
				DreamTrackerID: "tracker-1",
				UserID:         "user-1",
				ProgramID:      "program-1",
				Title:          "Target A",
				Status:         model.DreamTrackerStatusActive,
				CreatedAt:      now,
				UpdatedAt:      now,
				SourceType:     "MANUAL",
			},
			ProgramInfo: model.DreamTrackerProgramInfo{
				ProgramID: "program-1",
			},
		},
	}
	svc := service.NewDreamTrackerServiceWithDeps(repo, nil)

	detail, err := svc.GetDreamTrackerDetail(context.Background(), "user-1", "tracker-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if detail.DreamTracker.DreamTrackerID != "tracker-1" {
		t.Fatalf("expected tracker-1 got %s", detail.DreamTracker.DreamTrackerID)
	}
	if detail.ProgramInfo.ProgramID != "program-1" {
		t.Fatalf("unexpected program info: %+v", detail.ProgramInfo)
	}
}

func TestSubmitDreamRequirementService(t *testing.T) {
	repo := &fakeDreamTrackerRepo{
		requirement: model.DreamRequirementStatus{
			DreamReqStatusID: "req-status-1",
			DreamTrackerID:   "tracker-1",
			ReqCatalogID:     "req-1",
			Status:           model.DreamRequirementStatusNotUploaded,
		},
		document: model.Document{
			DocumentID: "doc-1",
			UserID:     "user-1",
			PublicURL:  "https://example.com/doc.pdf",
			MIMEType:   "application/pdf",
		},
	}
	aiClient := &fakeDreamTrackerAIClient{
		reviewResponse: dto.DreamRequirementReviewResponse{
			Status:     "VERIFIED",
			AIStatus:   "COMPLETED",
			AIMessages: []string{"document looks valid"},
		},
	}
	svc := service.NewDreamTrackerServiceWithDeps(repo, aiClient)

	output, err := svc.SubmitDreamRequirement(context.Background(), service.SubmitDreamRequirementInput{
		UserID:           "user-1",
		DreamReqStatusID: "req-status-1",
		DocumentID:       "doc-1",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if output.Requirement.Status != model.DreamRequirementStatusValue("VERIFIED") {
		t.Fatalf("expected VERIFIED got %q", output.Requirement.Status)
	}
	if len(output.AIMessages) != 1 || output.AIMessages[0] != "document looks valid" {
		t.Fatalf("unexpected ai messages: %+v", output.AIMessages)
	}
	if repo.updated.DocumentID == nil || *repo.updated.DocumentID != "doc-1" {
		t.Fatal("expected updated document id")
	}
}

func TestNewDreamTrackerServiceUsesEnvAIClient(t *testing.T) {
	t.Setenv("AI_SERVICE_URL", "https://example.com")
	svc := service.NewDreamTrackerService(&fakeDreamTrackerRepo{})
	if svc == nil {
		t.Fatal("expected service")
	}
}

func TestReviewRequirementHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dream-tracker/requirements/review" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"VERIFIED","ai_status":"COMPLETED","ai_messages":["ok"]}`))
	}))
	defer server.Close()

	client := service.NewHTTPDreamTrackerAIClient(server.URL)
	resp, err := client.ReviewRequirement(context.Background(), dto.DreamRequirementReviewRequest{
		DreamReqStatusID: "req-1",
		DreamTrackerID:   "tracker-1",
		ReqCatalogID:     "catalog-1",
		DocumentID:       "doc-1",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Status != "VERIFIED" || len(resp.AIMessages) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestReviewRequirementHTTPClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	client := service.NewHTTPDreamTrackerAIClient(server.URL)
	_, err := client.ReviewRequirement(context.Background(), dto.DreamRequirementReviewRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSubmitDreamRequirementWithoutAIClientService(t *testing.T) {
	repo := &fakeDreamTrackerRepo{
		requirement: model.DreamRequirementStatus{
			DreamReqStatusID: "req-status-1",
			DreamTrackerID:   "tracker-1",
			ReqCatalogID:     "req-1",
			Status:           model.DreamRequirementStatusNotUploaded,
		},
		document: model.Document{DocumentID: "doc-1", UserID: "user-1", MIMEType: "application/pdf"},
	}
	svc := service.NewDreamTrackerServiceWithDeps(repo, nil)

	output, err := svc.SubmitDreamRequirement(context.Background(), service.SubmitDreamRequirementInput{
		UserID:           "user-1",
		DreamReqStatusID: "req-status-1",
		DocumentID:       "doc-1",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if output.Requirement.Status != model.DreamRequirementStatusUploaded {
		t.Fatalf("expected uploaded got %q", output.Requirement.Status)
	}
}

func TestSubmitDreamRequirementAIErrorService(t *testing.T) {
	repo := &fakeDreamTrackerRepo{
		requirement: model.DreamRequirementStatus{
			DreamReqStatusID: "req-status-1",
			DreamTrackerID:   "tracker-1",
			ReqCatalogID:     "req-1",
			Status:           model.DreamRequirementStatusNotUploaded,
		},
		document: model.Document{DocumentID: "doc-1", UserID: "user-1", MIMEType: "application/pdf"},
	}
	aiClient := &fakeDreamTrackerAIClient{reviewErr: errors.New("ai failed")}
	svc := service.NewDreamTrackerServiceWithDeps(repo, aiClient)

	output, err := svc.SubmitDreamRequirement(context.Background(), service.SubmitDreamRequirementInput{
		UserID:           "user-1",
		DreamReqStatusID: "req-status-1",
		DocumentID:       "doc-1",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if output.Requirement.AIStatus == nil || *output.Requirement.AIStatus != "FAILED" {
		t.Fatalf("expected failed ai status got %+v", output.Requirement.AIStatus)
	}
}

func TestSubmitDreamRequirementRejectsNonPDFService(t *testing.T) {
	repo := &fakeDreamTrackerRepo{
		requirement: model.DreamRequirementStatus{
			DreamReqStatusID: "req-status-1",
			DreamTrackerID:   "tracker-1",
			ReqCatalogID:     "req-1",
			Status:           model.DreamRequirementStatusNotUploaded,
		},
		document: model.Document{
			DocumentID:       "doc-1",
			UserID:           "user-1",
			OriginalFilename: "image.png",
			MIMEType:         "image/png",
		},
	}
	svc := service.NewDreamTrackerServiceWithDeps(repo, nil)

	_, err := svc.SubmitDreamRequirement(context.Background(), service.SubmitDreamRequirementInput{
		UserID:           "user-1",
		DreamReqStatusID: "req-status-1",
		DocumentID:       "doc-1",
	})
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestCreateDreamTrackerInvalidInputService(t *testing.T) {
	svc := service.NewDreamTrackerServiceWithDeps(&fakeDreamTrackerRepo{}, nil)
	_, err := svc.CreateDreamTracker(context.Background(), service.CreateDreamTrackerInput{})
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestGetDreamTrackerDetailInvalidInputService(t *testing.T) {
	svc := service.NewDreamTrackerServiceWithDeps(&fakeDreamTrackerRepo{}, nil)
	_, err := svc.GetDreamTrackerDetail(context.Background(), "", "")
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestFirstNonEmptyViaAISelectionService(t *testing.T) {
	repo := &fakeDreamTrackerRepo{
		requirement: model.DreamRequirementStatus{
			DreamReqStatusID: "req-status-1",
			DreamTrackerID:   "tracker-1",
			ReqCatalogID:     "req-1",
			Status:           model.DreamRequirementStatusNotUploaded,
		},
		document: model.Document{DocumentID: "doc-1", UserID: "user-1", MIMEType: "application/pdf", DokumenURL: "", PublicURL: "https://example.com/public.pdf"},
	}
	aiClient := &fakeDreamTrackerAIClient{
		reviewResponse: dto.DreamRequirementReviewResponse{
			Status:   "VERIFIED",
			AIStatus: "COMPLETED",
		},
	}
	svc := service.NewDreamTrackerServiceWithDeps(repo, aiClient)

	_, err := svc.SubmitDreamRequirement(context.Background(), service.SubmitDreamRequirementInput{
		UserID:           "user-1",
		DreamReqStatusID: "req-status-1",
		DocumentID:       "doc-1",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if aiClient.lastRequest.DocumentURL != "https://example.com/public.pdf" {
		t.Fatalf("expected fallback public url, got %q", aiClient.lastRequest.DocumentURL)
	}
}
