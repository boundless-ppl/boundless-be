package service_test

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
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
	createdDoc     model.Document
	createDocErr   error
	trackers       []model.DreamTracker
	trackersErr    error
	detail         repository.DreamTrackerDetail
	details        map[string]repository.DreamTrackerDetail
	detailErr      error
	document       model.Document
	documentErr    error
	reusableDoc    model.Document
	reusableFound  bool
	reusableErr    error
	requirement    model.DreamRequirementStatus
	requirementErr error
	updated        model.DreamRequirementStatus
	updateErr      error
	seed           repository.DreamTrackerSeed
	seedErr        error
}

func (f *fakeDreamTrackerRepo) CreateDreamTracker(ctx context.Context, tracker model.DreamTracker) (model.DreamTracker, error) {
	f.createdTracker = tracker
	if f.createErr != nil {
		return model.DreamTracker{}, f.createErr
	}
	return tracker, nil
}

func (f *fakeDreamTrackerRepo) FindDreamTrackersByUser(ctx context.Context, userID string) ([]model.DreamTracker, error) {
	return f.trackers, f.trackersErr
}

func (f *fakeDreamTrackerRepo) ResolveDreamTrackerSeed(ctx context.Context, programID *string, sourceRecResultID *string) (repository.DreamTrackerSeed, error) {
	if f.seedErr != nil {
		return repository.DreamTrackerSeed{}, f.seedErr
	}
	if f.seed.ProgramID != "" {
		return f.seed, nil
	}
	seed := repository.DreamTrackerSeed{ProgramID: "program-1", Title: "Target A"}
	if programID != nil && *programID != "" {
		seed.ProgramID = *programID
	}
	return seed, nil
}

func (f *fakeDreamTrackerRepo) CreateDocument(ctx context.Context, doc model.Document) (model.Document, error) {
	f.createdDoc = doc
	if f.createDocErr != nil {
		return model.Document{}, f.createDocErr
	}
	return doc, nil
}

func (f *fakeDreamTrackerRepo) FindDreamTrackerDetail(ctx context.Context, dreamTrackerID, userID string) (repository.DreamTrackerDetail, error) {
	if f.details != nil {
		detail, ok := f.details[dreamTrackerID]
		if !ok {
			return repository.DreamTrackerDetail{}, f.detailErr
		}
		return detail, f.detailErr
	}
	return f.detail, f.detailErr
}

func (f *fakeDreamTrackerRepo) FindDocumentByIDAndUser(ctx context.Context, documentID, userID string) (model.Document, error) {
	return f.document, f.documentErr
}

func (f *fakeDreamTrackerRepo) FindReusableDocumentByUserAndType(ctx context.Context, userID, documentType string) (model.Document, bool, error) {
	return f.reusableDoc, f.reusableFound, f.reusableErr
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

func TestListDreamTrackersService(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeDreamTrackerRepo{
		trackers: []model.DreamTracker{
			{DreamTrackerID: "tracker-1", UserID: "user-1"},
		},
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
			ProgramInfo: model.DreamTrackerProgramInfo{ProgramID: "program-1"},
		},
	}
	svc := service.NewDreamTrackerServiceWithDeps(repo, nil)

	items, err := svc.ListDreamTrackers(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(items) != 1 || items[0].DreamTracker.DreamTrackerID != "tracker-1" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestGetDreamTrackerDashboardSummaryService(t *testing.T) {
	now := time.Now().UTC()
	nextDeadline := now.Add(48 * time.Hour)
	repo := &fakeDreamTrackerRepo{
		trackers: []model.DreamTracker{
			{DreamTrackerID: "tracker-1", UserID: "user-1"},
			{DreamTrackerID: "tracker-2", UserID: "user-1"},
			{DreamTrackerID: "tracker-3", UserID: "user-1"},
		},
		details: map[string]repository.DreamTrackerDetail{
			"tracker-1": {
				DreamTracker: model.DreamTracker{DreamTrackerID: "tracker-1", UserID: "user-1", Status: model.DreamTrackerStatusActive},
				Requirements: []model.DreamRequirementDetail{
					{DreamRequirementStatus: model.DreamRequirementStatus{Status: model.DreamRequirementStatusUploaded}},
					{DreamRequirementStatus: model.DreamRequirementStatus{Status: model.DreamRequirementStatusNotUploaded}},
				},
				Milestones: []model.DreamKeyMilestone{{DeadlineDate: &nextDeadline}},
			},
			"tracker-2": {
				DreamTracker: model.DreamTracker{DreamTrackerID: "tracker-2", UserID: "user-1", Status: model.DreamTrackerStatusCompleted},
				Requirements: []model.DreamRequirementDetail{
					{DreamRequirementStatus: model.DreamRequirementStatus{Status: model.DreamRequirementStatusUploaded}},
				},
			},
			"tracker-3": {
				DreamTracker: model.DreamTracker{DreamTrackerID: "tracker-3", UserID: "user-1", Status: model.DreamTrackerStatusActive},
				Requirements: []model.DreamRequirementDetail{
					{DreamRequirementStatus: model.DreamRequirementStatus{Status: model.DreamRequirementStatusNotUploaded}},
				},
			},
		},
	}
	svc := service.NewDreamTrackerServiceWithDeps(repo, nil)

	summary, err := svc.GetDreamTrackerDashboardSummary(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if summary.TotalTrackers != 3 || summary.IncompleteTrackers != 2 || summary.CompletedTrackers != 1 || summary.NearDeadlineTrackers != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
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
		detail: repository.DreamTrackerDetail{
			Requirements: []model.DreamRequirementDetail{
				{
					DreamRequirementStatus: model.DreamRequirementStatus{
						DreamReqStatusID: "req-status-1",
						DreamTrackerID:   "tracker-1",
						ReqCatalogID:     "req-1",
					},
					RequirementKey: "passport_document",
				},
			},
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
			Meta: &dto.DreamRequirementReviewMeta{
				DocumentType:       "TRANSCRIPT",
				VerificationStatus: "VERIFIED",
				ConfidenceScore:    0.91,
			},
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
	if output.ReviewMeta == nil || output.ReviewMeta.DocumentType != "TRANSCRIPT" {
		t.Fatalf("unexpected review meta: %+v", output.ReviewMeta)
	}
	if aiClient.lastRequest.RequiredDocumentType != "PASSPORT" {
		t.Fatalf("expected required document type PASSPORT, got %q", aiClient.lastRequest.RequiredDocumentType)
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
		if r.URL.Path != "/verify-document" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "multipart/form-data") {
			t.Fatalf("expected multipart content type, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"document_type":"PASSPORT","verification_status":"VERIFIED","confidence_score":0.91,"user_message":"ok"}`))
	}))
	defer server.Close()

	client := service.NewHTTPDreamTrackerAIClient(server.URL)
	resp, err := client.ReviewRequirement(context.Background(), dto.DreamRequirementReviewRequest{
		DreamReqStatusID: "req-1",
		DreamTrackerID:   "tracker-1",
		ReqCatalogID:     "catalog-1",
		DocumentID:       "doc-1",
		FileName:         "passport.pdf",
		FileContent:      []byte("fake-file"),
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
	_, err := client.ReviewRequirement(context.Background(), dto.DreamRequirementReviewRequest{
		FileName:    "doc.pdf",
		FileContent: []byte("fake-file"),
	})
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

func TestSubmitDreamRequirementAcceptsImageDocumentService(t *testing.T) {
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

func TestListDreamTrackersInvalidInputService(t *testing.T) {
	svc := service.NewDreamTrackerServiceWithDeps(&fakeDreamTrackerRepo{}, nil)
	_, err := svc.ListDreamTrackers(context.Background(), "")
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestGetDreamTrackerDashboardSummaryInvalidInputService(t *testing.T) {
	svc := service.NewDreamTrackerServiceWithDeps(&fakeDreamTrackerRepo{}, nil)
	_, err := svc.GetDreamTrackerDashboardSummary(context.Background(), "")
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
		detail: repository.DreamTrackerDetail{
			Requirements: []model.DreamRequirementDetail{
				{
					DreamRequirementStatus: model.DreamRequirementStatus{
						DreamReqStatusID: "req-status-1",
						DreamTrackerID:   "tracker-1",
						ReqCatalogID:     "req-1",
					},
					RequirementLabel: "Bank Statement",
				},
			},
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
	if aiClient.lastRequest.RequiredDocumentType != "BANK_STATEMENT" {
		t.Fatalf("expected BANK_STATEMENT, got %q", aiClient.lastRequest.RequiredDocumentType)
	}
}

func TestGetGroupedDreamTrackersService(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeDreamTrackerRepo{
		trackers: []model.DreamTracker{
			{DreamTrackerID: "tracker-1", UserID: "user-1"},
		},
		detail: repository.DreamTrackerDetail{
			DreamTracker: model.DreamTracker{
				DreamTrackerID: "tracker-1",
				UserID:         "user-1",
				Title:          "University of Bristol",
				Status:         model.DreamTrackerStatusActive,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			Summary: model.DreamTrackerSummary{CompletionPercentage: 33},
			ProgramInfo: model.DreamTrackerProgramInfo{
				ProgramID:      "program-1",
				ProgramName:    stringPtr("MSc Computer Science"),
				UniversityName: stringPtr("University of Bristol"),
				AdmissionName:  stringPtr("Fall 2027"),
			},
			Fundings: []model.DreamTrackerFundingOption{{
				FundingID:    "funding-1",
				NamaBeasiswa: "LPDP Scholarship",
			}},
		},
	}
	svc := service.NewDreamTrackerServiceWithDeps(repo, nil)

	output, err := svc.GetGroupedDreamTrackers(context.Background(), "user-1", nil, true)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if output.DefaultSelectedDreamTrackerID == nil || *output.DefaultSelectedDreamTrackerID != "tracker-1" {
		t.Fatalf("unexpected default selection: %+v", output.DefaultSelectedDreamTrackerID)
	}
	if len(output.Universities) != 1 || output.Universities[0].UniversityName != "University of Bristol" {
		t.Fatalf("unexpected universities: %+v", output.Universities)
	}
	if len(output.Fundings) != 1 || output.Fundings[0].FundingName != "LPDP Scholarship" {
		t.Fatalf("unexpected fundings: %+v", output.Fundings)
	}
	if output.DefaultDetail == nil || output.DefaultDetail.DreamTracker.DreamTrackerID != "tracker-1" {
		t.Fatalf("unexpected default detail: %+v", output.DefaultDetail)
	}
}

func TestUploadDreamRequirementDocumentRequiresFileForNewUploadService(t *testing.T) {
	repo := &fakeDreamTrackerRepo{
		requirement: model.DreamRequirementStatus{
			DreamReqStatusID: "req-status-1",
			DreamTrackerID:   "tracker-1",
			ReqCatalogID:     "req-1",
			Status:           model.DreamRequirementStatusNotUploaded,
		},
	}
	svc := service.NewDreamTrackerServiceWithDeps(repo, nil)

	_, _, err := svc.UploadDreamRequirementDocument(context.Background(), service.UploadDreamRequirementDocumentInput{
		UserID:           "user-1",
		DreamReqStatusID: "req-status-1",
		DocumentType:     "KTP",
		ReuseIfExists:    true,
	})
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestUploadDreamRequirementDocumentSkipsCurrentVerifiedService(t *testing.T) {
	repo := &fakeDreamTrackerRepo{
		requirement: model.DreamRequirementStatus{
			DreamReqStatusID: "req-status-1",
			DreamTrackerID:   "tracker-1",
			ReqCatalogID:     "req-1",
			DocumentID:       stringPtr("doc-1"),
			Status:           model.DreamRequirementStatusVerified,
		},
		document: model.Document{
			DocumentID:   "doc-1",
			UserID:       "user-1",
			DocumentType: model.DocumentType("KTP"),
			UploadedAt:   time.Now().UTC(),
		},
	}
	svc := service.NewDreamTrackerServiceWithDeps(repo, nil)

	output, statusCode, err := svc.UploadDreamRequirementDocument(context.Background(), service.UploadDreamRequirementDocumentInput{
		UserID:           "user-1",
		DreamReqStatusID: "req-status-1",
		DocumentType:     "KTP",
		ReuseIfExists:    true,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", statusCode)
	}
	if output.Review.Status != "SKIPPED" || !output.Review.IsAlreadyVerified {
		t.Fatalf("unexpected review: %+v", output.Review)
	}
}

func TestUploadDreamRequirementDocumentAcceptsCommonImageExtensionsService(t *testing.T) {
	cases := []struct {
		name     string
		filename string
	}{
		{name: "png", filename: "ktp.png"},
		{name: "jpg", filename: "ktp.jpg"},
		{name: "jpeg", filename: "ktp.jpeg"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			header := makeFileHeader(t, "file", tc.filename, []byte("small image payload"))
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
					OriginalFilename: tc.filename,
					MIMEType:         "image/jpeg",
				},
			}
			storage := &fakeDocumentUploader{
				output: service.StoredObject{
					StoragePath: "/tmp/" + tc.filename,
					PublicURL:   "http://local/" + tc.filename,
					MIMEType:    "image/jpeg",
					SizeBytes:   int64(header.Size),
				},
			}
			svc := service.NewDreamTrackerServiceWithDeps(repo, nil, storage)

			output, statusCode, err := svc.UploadDreamRequirementDocument(context.Background(), service.UploadDreamRequirementDocumentInput{
				UserID:           "user-1",
				DreamReqStatusID: "req-status-1",
				DocumentType:     "KTP",
				File:             header,
			})
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if statusCode != http.StatusAccepted && statusCode != http.StatusCreated {
				t.Fatalf("unexpected status code %d", statusCode)
			}
			if output.Document == nil || output.Document.OriginalFilename != tc.filename {
				t.Fatalf("unexpected document: %+v", output.Document)
			}
		})
	}
}

func TestSubmitDreamRequirementInfersRecommendationLetterTypeService(t *testing.T) {
	repo := &fakeDreamTrackerRepo{
		requirement: model.DreamRequirementStatus{
			DreamReqStatusID: "req-status-1",
			DreamTrackerID:   "tracker-1",
			ReqCatalogID:     "req-1",
			Status:           model.DreamRequirementStatusUploaded,
		},
		document: model.Document{
			DocumentID:   "doc-1",
			UserID:       "user-1",
			DocumentType: model.DocumentType("RECOMMENDATION_LETTER"),
			PublicURL:    "https://example.com/lor.pdf",
			MIMEType:     "application/pdf",
		},
		detail: repository.DreamTrackerDetail{
			DreamTracker: model.DreamTracker{DreamTrackerID: "tracker-1", UserID: "user-1"},
			Requirements: []model.DreamRequirementDetail{{
				DreamRequirementStatus: model.DreamRequirementStatus{
					DreamReqStatusID: "req-status-1",
					DreamTrackerID:   "tracker-1",
					ReqCatalogID:     "req-1",
				},
				RequirementKey:   "surat_rekomendasi",
				RequirementLabel: "Surat Rekomendasi",
				RequirementCategory: "RECOMMENDATION",
			}},
		},
	}
	ai := &fakeDreamTrackerAIClient{
		reviewResponse: dto.DreamRequirementReviewResponse{
			Status:   "REJECTED",
			AIStatus: "COMPLETED",
		},
	}
	svc := service.NewDreamTrackerServiceWithDeps(repo, ai)

	_, err := svc.SubmitDreamRequirement(context.Background(), service.SubmitDreamRequirementInput{
		UserID:           "user-1",
		DreamReqStatusID: "req-status-1",
		DocumentID:       "doc-1",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if ai.lastRequest.RequiredDocumentType != "RECOMMENDATION_LETTER" {
		t.Fatalf("expected RECOMMENDATION_LETTER, got %q", ai.lastRequest.RequiredDocumentType)
	}
}

func TestUploadDreamRequirementDocumentCanonicalizesRecommendationLetterTypeService(t *testing.T) {
	header := makeFileHeader(t, "file", "lor.pdf", []byte("pdf payload"))
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
			OriginalFilename: "lor.pdf",
			MIMEType:         "application/pdf",
			PublicURL:        "https://example.com/lor.pdf",
		},
		detail: repository.DreamTrackerDetail{
			DreamTracker: model.DreamTracker{DreamTrackerID: "tracker-1", UserID: "user-1"},
			Requirements: []model.DreamRequirementDetail{{
				DreamRequirementStatus: model.DreamRequirementStatus{
					DreamReqStatusID: "req-status-1",
					DreamTrackerID:   "tracker-1",
					ReqCatalogID:     "req-1",
				},
				RequirementKey:   "surat_rekomendasi",
				RequirementLabel: "Surat Rekomendasi",
				RequirementCategory: "RECOMMENDATION",
			}},
		},
	}
	storage := &fakeDocumentUploader{
		output: service.StoredObject{
			StoragePath: "/tmp/lor.pdf",
			PublicURL:   "http://local/lor.pdf",
			MIMEType:    "application/pdf",
			SizeBytes:   int64(header.Size),
		},
	}
	ai := &fakeDreamTrackerAIClient{
		reviewResponse: dto.DreamRequirementReviewResponse{
			Status:   "REJECTED",
			AIStatus: "COMPLETED",
		},
	}
	svc := service.NewDreamTrackerServiceWithDeps(repo, ai, storage)

	_, _, err := svc.UploadDreamRequirementDocument(context.Background(), service.UploadDreamRequirementDocumentInput{
		UserID:           "user-1",
		DreamReqStatusID: "req-status-1",
		DocumentType:     "surat_rekomendasi",
		File:             header,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.createdDoc.DocumentType != model.DocumentType("RECOMMENDATION_LETTER") {
		t.Fatalf("expected canonical document type, got %q", repo.createdDoc.DocumentType)
	}
}

type fakeDocumentUploader struct {
	output service.StoredObject
	err    error
}

func (f *fakeDocumentUploader) Upload(ctx context.Context, input service.UploadInput) (service.StoredObject, error) {
	if f.err != nil {
		return service.StoredObject{}, f.err
	}
	return f.output, nil
}

func makeFileHeader(t *testing.T, fieldName, filename string, content []byte) *multipart.FileHeader {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(int64(len(content)) + 1024); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}
	_, header, err := req.FormFile(fieldName)
	if err != nil {
		t.Fatalf("get form file: %v", err)
	}
	return header
}

func stringPtr(value string) *string {
	return &value
}
