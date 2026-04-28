package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"
)

type fakeDreamTrackerRepo struct {
	seed    repository.DreamTrackerSeed
	seedErr error

	createdTracker model.DreamTracker
	createErr      error

	trackersByUser []model.DreamTracker
	trackersErr    error

	detail    repository.DreamTrackerDetail
	detailErr error

	document    model.Document
	documentErr error

	requirement    model.DreamRequirementStatus
	requirementErr error

	updateCalls []model.DreamRequirementStatus
	updateErr   error
}

func (f *fakeDreamTrackerRepo) CreateDreamTracker(ctx context.Context, tracker model.DreamTracker) (model.DreamTracker, error) {
	f.createdTracker = tracker
	if f.createErr != nil {
		return model.DreamTracker{}, f.createErr
	}
	return tracker, nil
}

func (f *fakeDreamTrackerRepo) FindDreamTrackersByUser(ctx context.Context, userID string) ([]model.DreamTracker, error) {
	if f.trackersErr != nil {
		return nil, f.trackersErr
	}
	return f.trackersByUser, nil
}

func (f *fakeDreamTrackerRepo) FindDreamTrackerDetail(ctx context.Context, dreamTrackerID, userID string) (repository.DreamTrackerDetail, error) {
	if f.detailErr != nil {
		return repository.DreamTrackerDetail{}, f.detailErr
	}
	return f.detail, nil
}

func (f *fakeDreamTrackerRepo) ResolveDreamTrackerSeed(ctx context.Context, programID *string, sourceRecResultID *string, scholarshipName *string) (repository.DreamTrackerSeed, error) {
	if f.seedErr != nil {
		return repository.DreamTrackerSeed{}, f.seedErr
	}
	return f.seed, nil
}

func (f *fakeDreamTrackerRepo) CreateDocument(ctx context.Context, doc model.Document) (model.Document, error) {
	return doc, nil
}

func (f *fakeDreamTrackerRepo) FindDocumentByIDAndUser(ctx context.Context, documentID, userID string) (model.Document, error) {
	if f.documentErr != nil {
		return model.Document{}, f.documentErr
	}
	return f.document, nil
}

func (f *fakeDreamTrackerRepo) FindReusableDocumentByUserAndType(ctx context.Context, userID, documentType string) (model.Document, bool, error) {
	return model.Document{}, false, nil
}

func (f *fakeDreamTrackerRepo) FindDreamRequirementStatusByIDAndUser(ctx context.Context, dreamReqStatusID, userID string) (model.DreamRequirementStatus, error) {
	if f.requirementErr != nil {
		return model.DreamRequirementStatus{}, f.requirementErr
	}
	return f.requirement, nil
}

func (f *fakeDreamTrackerRepo) UpdateDreamRequirementStatus(ctx context.Context, requirement model.DreamRequirementStatus) error {
	f.updateCalls = append(f.updateCalls, requirement)
	if f.updateErr != nil {
		return f.updateErr
	}
	return nil
}

type fakeDreamReviewer struct {
	resp dto.DreamRequirementReviewResponse
	err  error
}

func (f *fakeDreamReviewer) ReviewRequirement(ctx context.Context, req dto.DreamRequirementReviewRequest) (dto.DreamRequirementReviewResponse, error) {
	if f.err != nil {
		return dto.DreamRequirementReviewResponse{}, f.err
	}
	return f.resp, nil
}

type workflowStorageStub struct{}

func (workflowStorageStub) Upload(ctx context.Context, input UploadInput) (StoredObject, error) {
	return StoredObject{}, nil
}

func TestDreamTrackerServiceCreateDreamTrackerUsesSeed(t *testing.T) {
	repo := &fakeDreamTrackerRepo{
		seed: repository.DreamTrackerSeed{
			ProgramID:   "program-1",
			Title:       "Dream Program",
			AdmissionID: stringPtr("admission-1"),
			FundingID:   stringPtr("funding-1"),
		},
	}
	svc := NewDreamTrackerServiceWithDeps(repo, nil, workflowStorageStub{})

	out, err := svc.CreateDreamTracker(context.Background(), CreateDreamTrackerInput{
		UserID:          "user-1",
		SourceType:      "MANUAL",
		ScholarshipName: stringPtr("LPDP"),
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.DreamTracker.ProgramID != "program-1" || out.DreamTracker.Title != "Dream Program" {
		t.Fatalf("expected seeded tracker fields, got %#v", out.DreamTracker)
	}
	if repo.createdTracker.FundingID == nil || *repo.createdTracker.FundingID != "funding-1" {
		t.Fatalf("expected seeded funding id, got %#v", repo.createdTracker.FundingID)
	}
}

func TestDreamTrackerServiceCreateDreamTrackerInvalidInput(t *testing.T) {
	repo := &fakeDreamTrackerRepo{}
	svc := NewDreamTrackerServiceWithDeps(repo, nil, workflowStorageStub{})

	_, err := svc.CreateDreamTracker(context.Background(), CreateDreamTrackerInput{})
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestDreamTrackerServiceGetAndListDetail(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeDreamTrackerRepo{
		trackersByUser: []model.DreamTracker{{DreamTrackerID: "tracker-1"}},
		detail: repository.DreamTrackerDetail{
			DreamTracker: model.DreamTracker{
				DreamTrackerID: "tracker-1",
				UserID:         "user-1",
				Title:          "Dream",
				Status:         model.DreamTrackerStatusActive,
			},
			Requirements: []model.DreamRequirementDetail{
				{DreamRequirementStatus: model.DreamRequirementStatus{Status: model.DreamRequirementStatusUploaded}},
			},
			Milestones: []model.DreamKeyMilestone{{DeadlineDate: &now}},
		},
	}
	svc := NewDreamTrackerServiceWithDeps(repo, nil, workflowStorageStub{})

	detail, err := svc.GetDreamTrackerDetail(context.Background(), "user-1", "tracker-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if detail.Summary.TotalRequirements != 1 {
		t.Fatalf("expected summary to be built, got %#v", detail.Summary)
	}

	list, err := svc.ListDreamTrackers(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(list) != 1 || list[0].DreamTracker.DreamTrackerID != "tracker-1" {
		t.Fatalf("expected one tracker, got %#v", list)
	}
}

func TestDreamTrackerServiceGroupedAndDashboardSummary(t *testing.T) {
	now := time.Now().UTC()
	univ := "University A"
	repo := &fakeDreamTrackerRepo{
		trackersByUser: []model.DreamTracker{{DreamTrackerID: "tracker-1"}},
		detail: repository.DreamTrackerDetail{
			DreamTracker: model.DreamTracker{
				DreamTrackerID: "tracker-1",
				UserID:         "user-1",
				Title:          "Dream",
				Status:         model.DreamTrackerStatusActive,
			},
			ProgramInfo: model.DreamTrackerProgramInfo{
				UniversityName: &univ,
			},
			Requirements: []model.DreamRequirementDetail{
				{DreamRequirementStatus: model.DreamRequirementStatus{Status: model.DreamRequirementStatusUploaded}},
			},
			Milestones: []model.DreamKeyMilestone{{DeadlineDate: &now}},
		},
	}
	svc := NewDreamTrackerServiceWithDeps(repo, nil, workflowStorageStub{})

	grouped, err := svc.GetGroupedDreamTrackers(context.Background(), "user-1", nil, false)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(grouped.Universities) != 1 {
		t.Fatalf("expected grouped universities, got %#v", grouped)
	}

	summary, err := svc.GetDreamTrackerDashboardSummary(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if summary.TotalTrackers != 1 {
		t.Fatalf("expected total trackers 1, got %#v", summary)
	}
}

func TestDreamTrackerServiceGetDocumentDetail(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeDreamTrackerRepo{
		document: model.Document{
			DocumentID: "doc-1",
			UserID:     "user-1",
			UploadedAt: now,
		},
	}
	svc := NewDreamTrackerServiceWithDeps(repo, nil, workflowStorageStub{})

	doc, err := svc.GetDocumentDetail(context.Background(), "user-1", "doc-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if doc.DocumentID != "doc-1" {
		t.Fatalf("expected doc-1, got %#v", doc)
	}
}

func TestDreamTrackerServiceSubmitRequirementWithoutAI(t *testing.T) {
	repo := &fakeDreamTrackerRepo{
		requirement: model.DreamRequirementStatus{
			DreamReqStatusID: "req-1",
			DreamTrackerID:   "tracker-1",
			ReqCatalogID:     "catalog-1",
			Status:           model.DreamRequirementStatusNotUploaded,
		},
		document: model.Document{
			DocumentID:   "doc-1",
			UserID:       "user-1",
			DocumentType: model.DocumentType("passport"),
		},
	}
	svc := NewDreamTrackerServiceWithDeps(repo, nil, workflowStorageStub{})

	out, err := svc.SubmitDreamRequirement(context.Background(), SubmitDreamRequirementInput{
		UserID:           "user-1",
		DreamReqStatusID: "req-1",
		DocumentID:       "doc-1",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Requirement.Status != model.DreamRequirementStatusUploaded {
		t.Fatalf("expected uploaded status, got %#v", out.Requirement.Status)
	}
	if len(repo.updateCalls) != 1 {
		t.Fatalf("expected one update call, got %d", len(repo.updateCalls))
	}
}

func TestDreamTrackerServiceSubmitRequirementAIErrorFallback(t *testing.T) {
	repo := &fakeDreamTrackerRepo{
		requirement: model.DreamRequirementStatus{
			DreamReqStatusID: "req-1",
			DreamTrackerID:   "tracker-1",
			ReqCatalogID:     "catalog-1",
			Status:           model.DreamRequirementStatusNotUploaded,
		},
		document: model.Document{
			DocumentID:   "doc-1",
			UserID:       "user-1",
			DocumentType: model.DocumentType("passport"),
		},
		detailErr: errors.New("ignore detail lookup"),
	}
	reviewer := &fakeDreamReviewer{err: errors.New("ai timeout")}
	svc := NewDreamTrackerServiceWithDeps(repo, reviewer, workflowStorageStub{})

	out, err := svc.SubmitDreamRequirement(context.Background(), SubmitDreamRequirementInput{
		UserID:           "user-1",
		DreamReqStatusID: "req-1",
		DocumentID:       "doc-1",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Requirement.Status != model.DreamRequirementStatusNeedsReview {
		t.Fatalf("expected needs review status after AI error, got %#v", out.Requirement.Status)
	}
	if len(repo.updateCalls) < 2 {
		t.Fatalf("expected at least two update calls, got %d", len(repo.updateCalls))
	}
}

func TestDreamTrackerServiceUploadRequirementDocumentInvalidInput(t *testing.T) {
	repo := &fakeDreamTrackerRepo{}
	svc := NewDreamTrackerServiceWithDeps(repo, nil, workflowStorageStub{})

	_, status, err := svc.UploadDreamRequirementDocument(context.Background(), UploadDreamRequirementDocumentInput{})
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
	if status != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, status)
	}
}

func TestDreamTrackerServiceUploadRequirementDocumentReuseVerified(t *testing.T) {
	docID := "doc-1"
	now := time.Now().UTC()
	repo := &fakeDreamTrackerRepo{
		requirement: model.DreamRequirementStatus{
			DreamReqStatusID: "req-1",
			DreamTrackerID:   "tracker-1",
			ReqCatalogID:     "catalog-1",
			DocumentID:       &docID,
			Status:           model.DreamRequirementStatusVerified,
		},
		document: model.Document{
			DocumentID:   docID,
			UserID:       "user-1",
			DocumentType: model.DocumentType("passport"),
			UploadedAt:   now,
		},
	}
	svc := NewDreamTrackerServiceWithDeps(repo, nil, workflowStorageStub{})

	out, status, err := svc.UploadDreamRequirementDocument(context.Background(), UploadDreamRequirementDocumentInput{
		UserID:           "user-1",
		DreamReqStatusID: "req-1",
		DocumentType:     "passport",
		ReuseIfExists:    true,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if out.Document == nil || out.Document.DocumentID != docID {
		t.Fatalf("expected reused existing doc, got %#v", out.Document)
	}
	if out.Review.Source != "SKIPPED_ALREADY_VERIFIED" || !out.Review.IsReused {
		t.Fatalf("unexpected review payload: %#v", out.Review)
	}
}
