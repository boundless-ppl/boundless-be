package service_test

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"
)

type fakeRecommendationRepo struct {
	createdDocument       model.Document
	createDocumentErr     error
	findDocumentByIDUser  map[string]model.Document
	findDocumentByIDError error

	createSubmissionParams repository.CreateSubmissionParams
	createSubmissionErr    error
	updatedStatuses        []model.RecommendationStatus
	createdResultRows      []model.RecommendationResult

	detail                     repository.SubmissionDetail
	latestTranscriptDetail     repository.SubmissionDetail
	findLatestTranscriptErr    error
	programMatches             []repository.RecommendationProgramMatch
	recommendationCountryCodes []string
	allowedCandidates          []repository.RecommendationAllowedCandidate
	scholarshipMatches         []repository.RecommendationScholarshipMatch
}

func (f *fakeRecommendationRepo) CreateDocument(ctx context.Context, doc model.Document) (model.Document, error) {
	f.createdDocument = doc
	if f.createDocumentErr != nil {
		return model.Document{}, f.createDocumentErr
	}
	return doc, nil
}

func (f *fakeRecommendationRepo) FindDocumentByIDAndUser(ctx context.Context, documentID, userID string) (model.Document, error) {
	if f.findDocumentByIDError != nil {
		return model.Document{}, f.findDocumentByIDError
	}
	doc, ok := f.findDocumentByIDUser[documentID]
	if !ok || doc.UserID != userID {
		return model.Document{}, errs.ErrDocumentNotFound
	}
	return doc, nil
}

func (f *fakeRecommendationRepo) CreateSubmission(ctx context.Context, params repository.CreateSubmissionParams) (model.RecommendationSubmission, error) {
	f.createSubmissionParams = params
	if f.createSubmissionErr != nil {
		return model.RecommendationSubmission{}, f.createSubmissionErr
	}
	return params.Submission, nil
}

func (f *fakeRecommendationRepo) UpdateSubmissionStatus(ctx context.Context, submissionID, userID string, status model.RecommendationStatus) error {
	f.updatedStatuses = append(f.updatedStatuses, status)
	return nil
}

func (f *fakeRecommendationRepo) CreateResultSet(ctx context.Context, submissionID string, generatedAt time.Time, results []model.RecommendationResult) (model.RecommendationResultSet, error) {
	f.createdResultRows = append(f.createdResultRows, results...)
	return model.RecommendationResultSet{}, nil
}

func (f *fakeRecommendationRepo) FindSubmissionDetail(ctx context.Context, submissionID, userID string) (repository.SubmissionDetail, error) {
	return f.detail, nil
}

func (f *fakeRecommendationRepo) FindLatestCompletedSubmissionByTranscriptDocument(ctx context.Context, userID, documentID string) (repository.SubmissionDetail, error) {
	if f.findLatestTranscriptErr != nil {
		return repository.SubmissionDetail{}, f.findLatestTranscriptErr
	}
	return f.latestTranscriptDetail, nil
}

func (f *fakeRecommendationRepo) FindMatchingPrograms(ctx context.Context, lookups []repository.RecommendationProgramLookup) ([]repository.RecommendationProgramMatch, error) {
	return f.programMatches, nil
}

func (f *fakeRecommendationRepo) ListRecommendationCountryCodes(ctx context.Context) ([]string, error) {
	return f.recommendationCountryCodes, nil
}

func (f *fakeRecommendationRepo) FindRecommendationAllowedCandidates(ctx context.Context, preferredCountryCodes []string, limit int) ([]repository.RecommendationAllowedCandidate, error) {
	return f.allowedCandidates, nil
}

func (f *fakeRecommendationRepo) FindScholarshipMatches(ctx context.Context, programIDs []string) ([]repository.RecommendationScholarshipMatch, error) {
	return f.scholarshipMatches, nil
}

func makeFileHeader(t *testing.T, fieldName, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}
	files := req.MultipartForm.File[fieldName]
	if len(files) == 0 {
		t.Fatal("missing multipart file header")
	}
	return files[0]
}

func ptrString(value string) *string {
	return &value
}

func TestCreateSubmissionServiceRejectsWhenNoDocument(t *testing.T) {
	repo := &fakeRecommendationRepo{}
	svc := service.NewRecommendationService(repo)

	_, err := svc.CreateSubmission(context.Background(), service.CreateSubmissionInput{UserID: "user-1"})
	if err != errs.ErrNoDocumentProvided {
		t.Fatalf("expected %v, got %v", errs.ErrNoDocumentProvided, err)
	}
}

func TestCreateSubmissionServiceRejectsUnsupportedFileType(t *testing.T) {
	repo := &fakeRecommendationRepo{}
	svc := service.NewRecommendationService(repo)
	header := makeFileHeader(t, "transcript", "transcript.txt", []byte("not allowed"))

	_, err := svc.UploadDocument(context.Background(), service.UploadDocumentInput{
		UserID:       "user-1",
		DocumentType: model.DocumentTypeTranscript,
		File:         header,
	})
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestCreateSubmissionServiceRejectsFileLargerThan5MB(t *testing.T) {
	repo := &fakeRecommendationRepo{}
	svc := service.NewRecommendationService(repo)
	header := makeFileHeader(t, "cv", "cv.pdf", []byte("small"))
	header.Size = service.MaxDocumentSizeBytes + 1

	_, err := svc.UploadDocument(context.Background(), service.UploadDocumentInput{
		UserID:       "user-1",
		DocumentType: model.DocumentTypeCV,
		File:         header,
	})
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestUploadDocumentServiceSuccess(t *testing.T) {
	t.Setenv("AI_SERVICE_URL", "")
	t.Setenv("DOCUMENT_PUBLIC_BASE_URL", "")
	t.Setenv("DOCUMENT_STORAGE_DIR", t.TempDir())

	repo := &fakeRecommendationRepo{}
	svc := service.NewRecommendationService(repo)
	header := makeFileHeader(t, "transcript", "transcript.pdf", []byte("%PDF-1.7 content"))

	out, err := svc.UploadDocument(context.Background(), service.UploadDocumentInput{
		UserID:       "user-1",
		DocumentType: model.DocumentTypeTranscript,
		File:         header,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Document.DocumentType != model.DocumentTypeTranscript {
		t.Fatalf("expected transcript type, got %s", out.Document.DocumentType)
	}
	if out.Document.SizeBytes <= 0 {
		t.Fatal("expected positive stored file size")
	}
	if _, err := os.Stat(out.Document.StoragePath); err != nil {
		t.Fatalf("expected uploaded file on disk, got %v", err)
	}
	if filepath.Ext(out.Document.OriginalFilename) != ".pdf" {
		t.Fatalf("expected pdf filename, got %s", out.Document.OriginalFilename)
	}
}

func TestCreateSubmissionServiceSuccessDraftMode(t *testing.T) {
	t.Setenv("AI_SERVICE_URL", "")

	transcriptID := "doc-transcript-1"
	repo := &fakeRecommendationRepo{
		findDocumentByIDUser: map[string]model.Document{
			transcriptID: {
				DocumentID:       transcriptID,
				UserID:           "user-1",
				OriginalFilename: "transcript.pdf",
				PublicURL:        "http://local/transcript.pdf",
				DocumentType:     model.DocumentTypeTranscript,
			},
		},
	}
	svc := service.NewRecommendationService(repo)

	out, err := svc.CreateSubmission(context.Background(), service.CreateSubmissionInput{
		UserID:               "user-1",
		TranscriptDocumentID: &transcriptID,
		Preferences: []dto.PreferenceInput{
			{Key: "country", Value: "Japan"},
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Status != model.RecommendationStatusDraft {
		t.Fatalf("expected draft status, got %s", out.Status)
	}
}

func TestUploadDocumentServiceAllowsSupportedExtensions(t *testing.T) {
	t.Setenv("DOCUMENT_STORAGE_DIR", t.TempDir())

	tests := []struct {
		name     string
		filename string
	}{
		{name: "pdf", filename: "doc.pdf"},
		{name: "png", filename: "doc.png"},
		{name: "jpg", filename: "doc.jpg"},
		{name: "jpeg", filename: "doc.jpeg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRecommendationRepo{}
			svc := service.NewRecommendationService(repo)
			header := makeFileHeader(t, "file", tt.filename, []byte("dummy content"))

			_, err := svc.UploadDocument(context.Background(), service.UploadDocumentInput{
				UserID:       "user-1",
				DocumentType: model.DocumentTypeTranscript,
				File:         header,
			})
			if err != nil {
				t.Fatalf("expected nil error for %s, got %v", tt.filename, err)
			}
		})
	}
}

type fakeRecommendationAIClient struct {
	profileResponse       dto.GlobalMatchAIRecommendationResponse
	transcriptResponse    dto.GlobalMatchAIRecommendationResponse
	cvResponse            dto.GlobalMatchAIRecommendationResponse
	err                   error
	lastProfileRequest    dto.AIProfileRecommendationRequest
	lastTranscriptRequest dto.AITranscriptRecommendationRequest
	lastCVRequest         dto.AICVRecommendationRequest
}

func (f *fakeRecommendationAIClient) RecommendProfile(ctx context.Context, req dto.AIProfileRecommendationRequest) (dto.GlobalMatchAIRecommendationResponse, error) {
	f.lastProfileRequest = req
	if f.err != nil {
		return dto.GlobalMatchAIRecommendationResponse{}, f.err
	}
	return f.profileResponse, nil
}

func (f *fakeRecommendationAIClient) RecommendTranscript(ctx context.Context, req dto.AITranscriptRecommendationRequest) (dto.GlobalMatchAIRecommendationResponse, error) {
	f.lastTranscriptRequest = req
	if f.err != nil {
		return dto.GlobalMatchAIRecommendationResponse{}, f.err
	}
	return f.transcriptResponse, nil
}

func (f *fakeRecommendationAIClient) RecommendCV(ctx context.Context, req dto.AICVRecommendationRequest) (dto.GlobalMatchAIRecommendationResponse, error) {
	f.lastCVRequest = req
	if f.err != nil {
		return dto.GlobalMatchAIRecommendationResponse{}, f.err
	}
	return f.cvResponse, nil
}

func TestCreateProfileRecommendationServiceSuccess(t *testing.T) {
	t.Setenv("DOCUMENT_STORAGE_DIR", t.TempDir())

	repo := &fakeRecommendationRepo{}
	repo.programMatches = []repository.RecommendationProgramMatch{{
		UniversityName: "University A",
		ProgramName:    "Computer Science",
		ProgramID:      "program-university-a-cs",
	}}
	repo.recommendationCountryCodes = []string{"JP", "SG"}
	repo.allowedCandidates = []repository.RecommendationAllowedCandidate{{
		ProgramID:             "program-university-a-cs",
		ProgramName:           "Computer Science",
		UniversityName:        "University A",
		CountryCode:           "JP",
		OfficialProgramURL:    "https://unia.example/cs",
		OfficialUniversityURL: "https://unia.example",
	}}
	aiClient := &fakeRecommendationAIClient{
		profileResponse: dto.GlobalMatchAIRecommendationResponse{
			SelectionReasoning: "strong fit",
			TopRecommendations: []dto.GlobalMatchAITopRecommendationResponse{
				{
					Rank:                       1,
					ProgramID:                  ptrString("program-university-a-cs"),
					UniversityName:             "University A",
					ProgramName:                "Computer Science",
					Country:                    "Japan",
					FitScore:                   90,
					AdmissionChanceScore:       75,
					OverallRecommendationScore: 88,
					FitLevel:                   "high",
					AdmissionDifficulty:        "moderate",
					Overview:                   "overview",
					WhyThisUniversity:          "why university",
					WhyThisProgram:             "why program",
					PreferenceReasoning:        []string{"matches country"},
					MatchEvidence:              []string{"good grades"},
					ScholarshipRecommendations: []dto.GlobalMatchAIScholarshipRecommendationResponse{{
						ScholarshipName: "MEXT",
					}},
					Pros: []string{"strong lab"},
					Cons: []string{"competitive"},
				},
			},
		},
	}
	svc := service.NewRecommendationServiceWithDeps(repo, service.NewLocalDocumentStorage(t.TempDir(), ""), aiClient)

	out, err := svc.CreateProfileRecommendation(context.Background(), "user-1", dto.CreateProfileRecommendationRequest{
		TranscriptFile: makeFileHeader(t, "transcript_file", "transcript.pdf", []byte("%PDF-1.7 transcript")),
		CVFile:         makeFileHeader(t, "cv_file", "cv.pdf", []byte("%PDF-1.7 cv")),
		RecommendationPreferenceInput: dto.RecommendationPreferenceInput{
			Countries:   []string{"Japan"},
			DegreeLevel: "Master",
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Status != model.RecommendationStatusCompleted {
		t.Fatalf("expected completed, got %s", out.Status)
	}
	if out.Result == nil || len(out.Result.TopRecommendations) != 1 {
		t.Fatalf("expected AI result, got %#v", out.Result)
	}
	if len(repo.createdResultRows) != 1 {
		t.Fatalf("expected one result row, got %d", len(repo.createdResultRows))
	}
	if repo.createdResultRows[0].ProgramID == nil || *repo.createdResultRows[0].ProgramID != "program-university-a-cs" {
		t.Fatalf("expected persisted program_id, got %#v", repo.createdResultRows[0].ProgramID)
	}
	if out.Result.TopRecommendations[0].ProgramID == nil || *out.Result.TopRecommendations[0].ProgramID != "program-university-a-cs" {
		t.Fatalf("expected response program_id, got %#v", out.Result.TopRecommendations[0].ProgramID)
	}
	if len(aiClient.lastProfileRequest.AllowedCandidates) != 1 {
		t.Fatalf("expected allowed candidates to be forwarded, got %#v", aiClient.lastProfileRequest.AllowedCandidates)
	}
	if aiClient.lastProfileRequest.AllowedCandidates[0].Country != "Japan" {
		t.Fatalf("expected country code to be converted to display name, got %#v", aiClient.lastProfileRequest.AllowedCandidates[0])
	}
	if len(repo.createSubmissionParams.Preferences) != 2 {
		t.Fatalf("expected flattened preferences, got %d", len(repo.createSubmissionParams.Preferences))
	}
	if len(repo.updatedStatuses) == 0 || repo.updatedStatuses[len(repo.updatedStatuses)-1] != model.RecommendationStatusCompleted {
		t.Fatalf("expected completed status update, got %#v", repo.updatedStatuses)
	}
}

func TestCreateTranscriptRecommendationServiceMarksFailedWhenAIRequestFails(t *testing.T) {
	repo := &fakeRecommendationRepo{}
	aiClient := &fakeRecommendationAIClient{err: errors.New("boom")}
	svc := service.NewRecommendationServiceWithDeps(repo, service.NewLocalDocumentStorage(t.TempDir(), ""), aiClient)

	_, err := svc.CreateTranscriptRecommendation(context.Background(), "user-1", dto.CreateTranscriptRecommendationRequest{
		TranscriptFile: makeFileHeader(t, "transcript_file", "transcript.pdf", []byte("%PDF-1.7 transcript")),
	})
	if !errors.Is(err, errs.ErrExternalService) {
		t.Fatalf("expected %v, got %v", errs.ErrExternalService, err)
	}
	if got := err.Error(); got == "" || !bytes.Contains([]byte(got), []byte("boom")) {
		t.Fatalf("expected wrapped upstream error, got %q", got)
	}
	if len(repo.updatedStatuses) == 0 || repo.updatedStatuses[0] != model.RecommendationStatusFailed {
		t.Fatalf("expected failed status update, got %#v", repo.updatedStatuses)
	}
}

func TestCreateTranscriptRecommendationServiceReusesCompletedSubmissionByDocumentID(t *testing.T) {
	documentID := "doc-transcript-1"
	repo := &fakeRecommendationRepo{
		findDocumentByIDUser: map[string]model.Document{
			documentID: {
				DocumentID:   documentID,
				UserID:       "user-1",
				DocumentType: model.DocumentTypeTranscript,
			},
		},
		latestTranscriptDetail: repository.SubmissionDetail{
			Submission: model.RecommendationSubmission{
				RecSubmissionID: "submission-1",
				UserID:          "user-1",
				Status:          model.RecommendationStatusCompleted,
			},
			Preferences: []model.RecommendationPreference{
				{PreferenceKey: "countries", PreferenceValue: "Japan"},
				{PreferenceKey: "degree_level", PreferenceValue: "Master"},
			},
			LatestResultSet: &model.RecommendationResultSet{
				ResultSetID: "result-set-1",
			},
			Results: []model.RecommendationResult{{
				RecResultID:                "rec-1",
				ResultSetID:                "result-set-1",
				RankNo:                     1,
				UniversityName:             "University A",
				ProgramName:                "Computer Science",
				Country:                    "Japan",
				FitScore:                   90,
				OverallRecommendationScore: 88,
				FitLevel:                   "high",
				Overview:                   "overview",
				WhyThisUniversity:          "why university",
				WhyThisProgram:             "why program",
				ReasonSummary:              "matches profile",
				ProsJSON:                   `["strong labs"]`,
				ConsJSON:                   `["competitive"]`,
			}},
		},
	}
	svc := service.NewRecommendationServiceWithDeps(repo, service.NewLocalDocumentStorage(t.TempDir(), ""), &fakeRecommendationAIClient{})

	out, err := svc.CreateTranscriptRecommendation(context.Background(), "user-1", dto.CreateTranscriptRecommendationRequest{
		TranscriptDocumentID: &documentID,
		RecommendationPreferenceInput: dto.RecommendationPreferenceInput{
			Countries:   []string{"Japan"},
			DegreeLevel: "Master",
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.SubmissionID != "submission-1" {
		t.Fatalf("expected reused submission, got %s", out.SubmissionID)
	}
	if out.Result == nil || len(out.Result.TopRecommendations) != 1 {
		t.Fatalf("expected reused result, got %#v", out.Result)
	}
	if repo.createdDocument.DocumentID != "" {
		t.Fatalf("expected no new document to be created, got %#v", repo.createdDocument)
	}
	if len(repo.updatedStatuses) != 0 {
		t.Fatalf("expected no status updates for reuse, got %#v", repo.updatedStatuses)
	}
}

func TestCreateCVRecommendationServiceSuccess(t *testing.T) {
	repo := &fakeRecommendationRepo{
		programMatches: []repository.RecommendationProgramMatch{{
			UniversityName: "University C",
			ProgramName:    "Data Science",
			ProgramID:      "program-university-c-data-science",
		}},
		recommendationCountryCodes: []string{"SG"},
		allowedCandidates: []repository.RecommendationAllowedCandidate{{
			ProgramID:             "program-university-c-data-science",
			ProgramName:           "Data Science",
			UniversityName:        "University C",
			CountryCode:           "SG",
			OfficialProgramURL:    "https://unic.example/data-science",
			OfficialUniversityURL: "https://unic.example",
		}},
	}
	aiClient := &fakeRecommendationAIClient{
		cvResponse: dto.GlobalMatchAIRecommendationResponse{
			SelectionReasoning: "cv fit",
			TopRecommendations: []dto.GlobalMatchAITopRecommendationResponse{{
				Rank:                       1,
				ProgramID:                  ptrString("program-university-c-data-science"),
				UniversityName:             "University C",
				ProgramName:                "Data Science",
				Country:                    "Singapore",
				FitScore:                   85,
				AdmissionChanceScore:       72,
				OverallRecommendationScore: 81,
				FitLevel:                   "high",
				AdmissionDifficulty:        "moderate",
				Overview:                   "overview",
				WhyThisUniversity:          "why university",
				WhyThisProgram:             "why program",
			}},
		},
	}
	svc := service.NewRecommendationServiceWithDeps(repo, service.NewLocalDocumentStorage(t.TempDir(), ""), aiClient)

	out, err := svc.CreateCVRecommendation(context.Background(), "user-1", dto.CreateCVRecommendationRequest{
		CVFile: makeFileHeader(t, "cv_file", "cv.pdf", []byte("%PDF-1.7 cv")),
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Status != model.RecommendationStatusCompleted {
		t.Fatalf("expected completed, got %s", out.Status)
	}
	if len(aiClient.lastCVRequest.AllowedCandidates) != 1 {
		t.Fatalf("expected allowed candidates to be forwarded, got %#v", aiClient.lastCVRequest.AllowedCandidates)
	}
}

func TestCreateProfileRecommendationServiceResolvesScholarshipLinks(t *testing.T) {
	repo := &fakeRecommendationRepo{
		recommendationCountryCodes: []string{"JP"},
		allowedCandidates: []repository.RecommendationAllowedCandidate{{
			ProgramID:             "program-a",
			ProgramName:           "Computer Science",
			UniversityName:        "University A",
			CountryCode:           "JP",
			OfficialProgramURL:    "https://unia.example/cs",
			OfficialUniversityURL: "https://unia.example",
		}},
		scholarshipMatches: []repository.RecommendationScholarshipMatch{{
			ProgramID:       "program-a",
			ScholarshipName: "JASSO Scholarship",
			FundingID:       "funding-1",
			AdmissionID:     "admission-1",
		}},
	}
	aiClient := &fakeRecommendationAIClient{
		profileResponse: dto.GlobalMatchAIRecommendationResponse{
			TopRecommendations: []dto.GlobalMatchAITopRecommendationResponse{{
				Rank:              1,
				ProgramID:         ptrString("program-a"),
				UniversityName:    "University A",
				ProgramName:       "Computer Science",
				Country:           "Japan",
				FitScore:          90,
				FitLevel:          "high",
				Overview:          "overview",
				WhyThisUniversity: "why university",
				WhyThisProgram:    "why program",
				ScholarshipRecommendations: []dto.GlobalMatchAIScholarshipRecommendationResponse{{
					ScholarshipName: "JASSO Scholarship",
					CoverageSummary: "partial support",
					Selectivity:     "moderate",
					EligibilityHint: "international student",
				}},
			}},
		},
	}
	svc := service.NewRecommendationServiceWithDeps(repo, service.NewLocalDocumentStorage(t.TempDir(), ""), aiClient)

	out, err := svc.CreateProfileRecommendation(context.Background(), "user-1", dto.CreateProfileRecommendationRequest{
		TranscriptFile: makeFileHeader(t, "transcript_file", "transcript.pdf", []byte("%PDF-1.7 transcript")),
		CVFile:         makeFileHeader(t, "cv_file", "cv.pdf", []byte("%PDF-1.7 cv")),
		RecommendationPreferenceInput: dto.RecommendationPreferenceInput{
			Countries: []string{"Japan"},
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Result == nil || len(out.Result.TopRecommendations) != 1 {
		t.Fatalf("expected recommendation result, got %#v", out.Result)
	}
	scholarships := out.Result.TopRecommendations[0].ScholarshipRecommendations
	if len(scholarships) != 1 {
		t.Fatalf("expected one scholarship, got %#v", scholarships)
	}
	if scholarships[0].FundingID == nil || *scholarships[0].FundingID != "funding-1" {
		t.Fatalf("expected funding_id to be resolved, got %#v", scholarships[0].FundingID)
	}
	if scholarships[0].AdmissionID == nil || *scholarships[0].AdmissionID != "admission-1" {
		t.Fatalf("expected admission_id to be resolved, got %#v", scholarships[0].AdmissionID)
	}
}

func TestCreateProfileRecommendationServiceResolvesScholarshipLinksWhenFundingMatchesButAdmissionsDiffer(t *testing.T) {
	repo := &fakeRecommendationRepo{
		recommendationCountryCodes: []string{"JP"},
		allowedCandidates: []repository.RecommendationAllowedCandidate{{
			ProgramID:             "program-a",
			ProgramName:           "Computer Science",
			UniversityName:        "University A",
			CountryCode:           "JP",
			OfficialProgramURL:    "https://unia.example/cs",
			OfficialUniversityURL: "https://unia.example",
		}},
		scholarshipMatches: []repository.RecommendationScholarshipMatch{
			{
				ProgramID:       "program-a",
				ScholarshipName: "JASSO Scholarship",
				FundingID:       "funding-1",
				AdmissionID:     "admission-1",
			},
			{
				ProgramID:       "program-a",
				ScholarshipName: "JASSO Scholarship",
				FundingID:       "funding-1",
				AdmissionID:     "admission-2",
			},
		},
	}
	aiClient := &fakeRecommendationAIClient{
		profileResponse: dto.GlobalMatchAIRecommendationResponse{
			TopRecommendations: []dto.GlobalMatchAITopRecommendationResponse{{
				Rank:              1,
				ProgramID:         ptrString("program-a"),
				UniversityName:    "University A",
				ProgramName:       "Computer Science",
				Country:           "Japan",
				FitScore:          90,
				FitLevel:          "high",
				Overview:          "overview",
				WhyThisUniversity: "why university",
				WhyThisProgram:    "why program",
				ScholarshipRecommendations: []dto.GlobalMatchAIScholarshipRecommendationResponse{{
					ScholarshipName: "JASSO Scholarship",
					CoverageSummary: "partial support",
					Selectivity:     "moderate",
					EligibilityHint: "international student",
				}},
			}},
		},
	}
	svc := service.NewRecommendationServiceWithDeps(repo, service.NewLocalDocumentStorage(t.TempDir(), ""), aiClient)

	out, err := svc.CreateProfileRecommendation(context.Background(), "user-1", dto.CreateProfileRecommendationRequest{
		TranscriptFile: makeFileHeader(t, "transcript_file", "transcript.pdf", []byte("%PDF-1.7 transcript")),
		CVFile:         makeFileHeader(t, "cv_file", "cv.pdf", []byte("%PDF-1.7 cv")),
		RecommendationPreferenceInput: dto.RecommendationPreferenceInput{
			Countries: []string{"Japan"},
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Result == nil || len(out.Result.TopRecommendations) != 1 {
		t.Fatalf("expected recommendation result, got %#v", out.Result)
	}
	scholarships := out.Result.TopRecommendations[0].ScholarshipRecommendations
	if len(scholarships) != 1 {
		t.Fatalf("expected one scholarship, got %#v", scholarships)
	}
	if scholarships[0].FundingID == nil || *scholarships[0].FundingID != "funding-1" {
		t.Fatalf("expected funding_id to be resolved, got %#v", scholarships[0].FundingID)
	}
	if scholarships[0].AdmissionID == nil || *scholarships[0].AdmissionID != "admission-1" {
		t.Fatalf("expected first admission_id to be resolved deterministically, got %#v", scholarships[0].AdmissionID)
	}
}

func TestCreateProfileRecommendationServiceFallsBackToOtherCountriesWhenPreferredOnlyOne(t *testing.T) {
	repo := &fakeRecommendationRepo{
		programMatches: []repository.RecommendationProgramMatch{
			{
				UniversityName: "University A",
				ProgramName:    "Computer Science",
				ProgramID:      "program-a",
			},
			{
				UniversityName: "University B",
				ProgramName:    "Computer Science",
				ProgramID:      "program-b",
			},
			{
				UniversityName: "University C",
				ProgramName:    "Computer Science",
				ProgramID:      "program-c",
			},
		},
	}
	aiClient := &fakeRecommendationAIClient{
		profileResponse: dto.GlobalMatchAIRecommendationResponse{
			TopRecommendations: []dto.GlobalMatchAITopRecommendationResponse{
				{
					Rank:              1,
					UniversityName:    "University B",
					ProgramName:       "Computer Science",
					Country:           "Singapore",
					FitScore:          87,
					FitLevel:          "high",
					Overview:          "overview",
					WhyThisUniversity: "why university",
					WhyThisProgram:    "why program",
				},
				{
					Rank:              2,
					UniversityName:    "University A",
					ProgramName:       "Computer Science",
					Country:           "Japan",
					FitScore:          90,
					FitLevel:          "high",
					Overview:          "overview",
					WhyThisUniversity: "why university",
					WhyThisProgram:    "why program",
				},
				{
					Rank:              3,
					UniversityName:    "University C",
					ProgramName:       "Computer Science",
					Country:           "Australia",
					FitScore:          84,
					FitLevel:          "high",
					Overview:          "overview",
					WhyThisUniversity: "why university",
					WhyThisProgram:    "why program",
				},
			},
		},
	}
	svc := service.NewRecommendationServiceWithDeps(repo, service.NewLocalDocumentStorage(t.TempDir(), ""), aiClient)

	out, err := svc.CreateProfileRecommendation(context.Background(), "user-1", dto.CreateProfileRecommendationRequest{
		TranscriptFile: makeFileHeader(t, "transcript_file", "transcript.pdf", []byte("%PDF-1.7 transcript")),
		CVFile:         makeFileHeader(t, "cv_file", "cv.pdf", []byte("%PDF-1.7 cv")),
		RecommendationPreferenceInput: dto.RecommendationPreferenceInput{
			Countries: []string{"Japan"},
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Result == nil || len(out.Result.TopRecommendations) != 3 {
		t.Fatalf("expected three recommendations, got %#v", out.Result)
	}
	if out.Result.TopRecommendations[0].Country != "Japan" {
		t.Fatalf("expected preferred country first, got %#v", out.Result.TopRecommendations)
	}
	if out.Result.TopRecommendations[1].Country == "Japan" {
		t.Fatalf("expected fallback from other countries after preferred result, got %#v", out.Result.TopRecommendations)
	}
	if out.Result.TopRecommendations[0].Rank != 1 || out.Result.TopRecommendations[1].Rank != 2 || out.Result.TopRecommendations[2].Rank != 3 {
		t.Fatalf("expected ranks to be recomputed, got %#v", out.Result.TopRecommendations)
	}
}

func TestCreateProfileRecommendationServiceKeepsPreferredCountriesAheadWhenMultipleMatches(t *testing.T) {
	repo := &fakeRecommendationRepo{
		programMatches: []repository.RecommendationProgramMatch{
			{
				UniversityName: "University A",
				ProgramName:    "Computer Science",
				ProgramID:      "program-a",
			},
			{
				UniversityName: "University B",
				ProgramName:    "Computer Science",
				ProgramID:      "program-b",
			},
			{
				UniversityName: "University C",
				ProgramName:    "Computer Science",
				ProgramID:      "program-c",
			},
		},
	}
	aiClient := &fakeRecommendationAIClient{
		profileResponse: dto.GlobalMatchAIRecommendationResponse{
			TopRecommendations: []dto.GlobalMatchAITopRecommendationResponse{
				{
					Rank:              1,
					UniversityName:    "University C",
					ProgramName:       "Computer Science",
					Country:           "Singapore",
					FitScore:          84,
					FitLevel:          "high",
					Overview:          "overview",
					WhyThisUniversity: "why university",
					WhyThisProgram:    "why program",
				},
				{
					Rank:              2,
					UniversityName:    "University A",
					ProgramName:       "Computer Science",
					Country:           "Japan",
					FitScore:          90,
					FitLevel:          "high",
					Overview:          "overview",
					WhyThisUniversity: "why university",
					WhyThisProgram:    "why program",
				},
				{
					Rank:              3,
					UniversityName:    "University B",
					ProgramName:       "Computer Science",
					Country:           "Japan",
					FitScore:          88,
					FitLevel:          "high",
					Overview:          "overview",
					WhyThisUniversity: "why university",
					WhyThisProgram:    "why program",
				},
			},
		},
	}
	svc := service.NewRecommendationServiceWithDeps(repo, service.NewLocalDocumentStorage(t.TempDir(), ""), aiClient)

	out, err := svc.CreateProfileRecommendation(context.Background(), "user-1", dto.CreateProfileRecommendationRequest{
		TranscriptFile: makeFileHeader(t, "transcript_file", "transcript.pdf", []byte("%PDF-1.7 transcript")),
		CVFile:         makeFileHeader(t, "cv_file", "cv.pdf", []byte("%PDF-1.7 cv")),
		RecommendationPreferenceInput: dto.RecommendationPreferenceInput{
			Countries: []string{"Japan"},
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.Result == nil || len(out.Result.TopRecommendations) != 3 {
		t.Fatalf("expected three recommendations, got %#v", out.Result)
	}
	if out.Result.TopRecommendations[0].Country != "Japan" || out.Result.TopRecommendations[1].Country != "Japan" {
		t.Fatalf("expected preferred-country recommendations to stay ahead, got %#v", out.Result.TopRecommendations)
	}
}

func TestCreateProfileRecommendationServiceFailsWhenNoResultMatchesCatalog(t *testing.T) {
	repo := &fakeRecommendationRepo{}
	aiClient := &fakeRecommendationAIClient{
		profileResponse: dto.GlobalMatchAIRecommendationResponse{
			TopRecommendations: []dto.GlobalMatchAITopRecommendationResponse{{
				Rank:              1,
				UniversityName:    "Imaginary University",
				ProgramName:       "Fictional Program",
				Country:           "Nowhere",
				FitScore:          90,
				FitLevel:          "high",
				Overview:          "overview",
				WhyThisUniversity: "why university",
				WhyThisProgram:    "why program",
			}},
		},
	}
	svc := service.NewRecommendationServiceWithDeps(repo, service.NewLocalDocumentStorage(t.TempDir(), ""), aiClient)

	_, err := svc.CreateProfileRecommendation(context.Background(), "user-1", dto.CreateProfileRecommendationRequest{
		TranscriptFile: makeFileHeader(t, "transcript_file", "transcript.pdf", []byte("%PDF-1.7 transcript")),
		CVFile:         makeFileHeader(t, "cv_file", "cv.pdf", []byte("%PDF-1.7 cv")),
	})
	if !errors.Is(err, errs.ErrExternalService) {
		t.Fatalf("expected %v, got %v", errs.ErrExternalService, err)
	}
	if len(repo.createdResultRows) != 0 {
		t.Fatalf("expected no persisted results, got %d", len(repo.createdResultRows))
	}
	if len(repo.updatedStatuses) == 0 || repo.updatedStatuses[len(repo.updatedStatuses)-1] != model.RecommendationStatusFailed {
		t.Fatalf("expected failed status update, got %#v", repo.updatedStatuses)
	}
}

func TestCreateCVRecommendationServiceRejectsMissingFile(t *testing.T) {
	svc := service.NewRecommendationServiceWithDeps(&fakeRecommendationRepo{}, service.NewLocalDocumentStorage(t.TempDir(), ""), &fakeRecommendationAIClient{})

	_, err := svc.CreateCVRecommendation(context.Background(), "user-1", dto.CreateCVRecommendationRequest{})
	if err != errs.ErrInvalidInput {
		t.Fatalf("expected %v, got %v", errs.ErrInvalidInput, err)
	}
}

func TestCreateProfileRecommendationServiceRejectsWhenAIClientMissing(t *testing.T) {
	svc := service.NewRecommendationServiceWithDeps(&fakeRecommendationRepo{}, service.NewLocalDocumentStorage(t.TempDir(), ""), nil)

	_, err := svc.CreateProfileRecommendation(context.Background(), "user-1", dto.CreateProfileRecommendationRequest{
		TranscriptFile: makeFileHeader(t, "transcript_file", "transcript.pdf", []byte("%PDF-1.7 transcript")),
		CVFile:         makeFileHeader(t, "cv_file", "cv.pdf", []byte("%PDF-1.7 cv")),
	})
	if err != errs.ErrExternalService {
		t.Fatalf("expected %v, got %v", errs.ErrExternalService, err)
	}
}
