package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"

	"github.com/google/uuid"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

const (
	MaxDocumentSizeBytes                int64 = 5 * 1024 * 1024
	recommendationAllowedCandidateLimit       = 30
)

var copyBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 32*1024)
		return &buf
	},
}

var allowedDocumentExtensions = map[string]struct{}{
	".pdf":  {},
	".png":  {},
	".jpg":  {},
	".jpeg": {},
}

type UploadInput struct {
	UserID       string
	DocumentType model.DocumentType
	Header       *multipart.FileHeader
}

type StoredObject struct {
	StoragePath string
	PublicURL   string
	SizeBytes   int64
	MIMEType    string
}

type DocumentStorage interface {
	Upload(ctx context.Context, input UploadInput) (StoredObject, error)
}

type UploadDocumentInput struct {
	UserID       string
	DocumentType model.DocumentType
	File         *multipart.FileHeader
}

type UploadDocumentOutput struct {
	Document model.Document
}

type CreateSubmissionInput struct {
	UserID               string
	TranscriptDocumentID *string
	CVDocumentID         *string
	Preferences          []dto.PreferenceInput
}

type CreateSubmissionOutput struct {
	SubmissionID string
	Status       model.RecommendationStatus
	ResultSetID  string
}

type CreateRecommendationWorkflowOutput struct {
	SubmissionID string
	Status       model.RecommendationStatus
	ResultSetID  string
	Result       *dto.GlobalMatchAIRecommendationResponse
}

type RecommendationService struct {
	repo        repository.RecommendationRepository
	storage     DocumentStorage
	aiClient    RecommendationAIClient
	hasAIClient bool
}

func NewRecommendationService(repo repository.RecommendationRepository) *RecommendationService {
	storage := mustBuildDocumentStorage()

	aiURL := strings.TrimSpace(os.Getenv("AI_SERVICE_URL"))
	service := &RecommendationService{
		repo:    repo,
		storage: storage,
	}
	if aiURL != "" {
		service.aiClient = NewHTTPRecommendationAIClient(aiURL)
		service.hasAIClient = true
	}
	return service
}

func NewRecommendationServiceWithDeps(
	repo repository.RecommendationRepository,
	storage DocumentStorage,
	aiClient RecommendationAIClient,
) *RecommendationService {
	return &RecommendationService{
		repo:        repo,
		storage:     storage,
		aiClient:    aiClient,
		hasAIClient: aiClient != nil,
	}
}

func (s *RecommendationService) UploadDocument(ctx context.Context, input UploadDocumentInput) (UploadDocumentOutput, error) {
	if input.UserID == "" {
		return UploadDocumentOutput{}, errs.ErrUnauthorized
	}
	if input.DocumentType != model.DocumentTypeTranscript && input.DocumentType != model.DocumentTypeCV {
		return UploadDocumentOutput{}, errs.ErrInvalidInput
	}
	if err := validateUploadHeader(input.File); err != nil {
		return UploadDocumentOutput{}, err
	}

	stored, err := s.storage.Upload(ctx, UploadInput{
		UserID:       input.UserID,
		DocumentType: input.DocumentType,
		Header:       input.File,
	})
	if err != nil {
		return UploadDocumentOutput{}, err
	}

	doc := model.Document{
		DocumentID:       uuid.NewString(),
		UserID:           input.UserID,
		OriginalFilename: input.File.Filename,
		StoragePath:      stored.StoragePath,
		PublicURL:        stored.PublicURL,
		MIMEType:         stored.MIMEType,
		SizeBytes:        stored.SizeBytes,
		DocumentType:     input.DocumentType,
		UploadedAt:       time.Now().UTC(),
	}

	created, err := s.repo.CreateDocument(ctx, doc)
	if err != nil {
		return UploadDocumentOutput{}, err
	}

	return UploadDocumentOutput{Document: created}, nil
}

func (s *RecommendationService) CreateSubmission(ctx context.Context, input CreateSubmissionInput) (CreateSubmissionOutput, error) {
	if input.UserID == "" {
		return CreateSubmissionOutput{}, errs.ErrUnauthorized
	}
	if input.TranscriptDocumentID == nil && input.CVDocumentID == nil {
		return CreateSubmissionOutput{}, errs.ErrNoDocumentProvided
	}

	now := time.Now().UTC()
	submissionID := uuid.NewString()

	var transcriptDoc *model.Document
	if input.TranscriptDocumentID != nil {
		doc, err := s.repo.FindDocumentByIDAndUser(ctx, *input.TranscriptDocumentID, input.UserID)
		if err != nil {
			return CreateSubmissionOutput{}, err
		}
		if doc.DocumentType != model.DocumentTypeTranscript {
			return CreateSubmissionOutput{}, errs.ErrInvalidInput
		}
		transcriptDoc = &doc
	}

	var cvDoc *model.Document
	if input.CVDocumentID != nil {
		doc, err := s.repo.FindDocumentByIDAndUser(ctx, *input.CVDocumentID, input.UserID)
		if err != nil {
			return CreateSubmissionOutput{}, err
		}
		if doc.DocumentType != model.DocumentTypeCV {
			return CreateSubmissionOutput{}, errs.ErrInvalidInput
		}
		cvDoc = &doc
	}

	preferences, err := buildLegacyPreferences(submissionID, now, input.Preferences)
	if err != nil {
		return CreateSubmissionOutput{}, err
	}

	submission := model.RecommendationSubmission{
		RecSubmissionID:      submissionID,
		UserID:               input.UserID,
		Status:               model.RecommendationStatusDraft,
		CreatedAt:            now,
		UpdatedAt:            now,
		TranscriptDocumentID: documentIDPtr(transcriptDoc),
		CVDocumentID:         documentIDPtr(cvDoc),
	}

	if _, err := s.repo.CreateSubmission(ctx, repository.CreateSubmissionParams{
		Submission:  submission,
		Preferences: preferences,
	}); err != nil {
		return CreateSubmissionOutput{}, err
	}

	return CreateSubmissionOutput{
		SubmissionID: submissionID,
		Status:       model.RecommendationStatusDraft,
	}, nil
}

func (s *RecommendationService) CreateProfileRecommendation(
	ctx context.Context,
	userID string,
	req dto.CreateProfileRecommendationRequest,
) (CreateRecommendationWorkflowOutput, error) {
	if req.TranscriptFile == nil || req.CVFile == nil {
		return CreateRecommendationWorkflowOutput{}, errs.ErrInvalidInput
	}

	return s.processRecommendation(
		ctx,
		userID,
		model.RecommendationModeProfile,
		&documentUploadRequest{documentType: model.DocumentTypeTranscript, header: req.TranscriptFile},
		&documentUploadRequest{documentType: model.DocumentTypeCV, header: req.CVFile},
		req.RecommendationPreferenceInput,
	)
}

func (s *RecommendationService) CreateTranscriptRecommendation(
	ctx context.Context,
	userID string,
	req dto.CreateTranscriptRecommendationRequest,
) (CreateRecommendationWorkflowOutput, error) {
	if req.TranscriptFile == nil && req.TranscriptDocumentID == nil {
		return CreateRecommendationWorkflowOutput{}, errs.ErrInvalidInput
	}

	if req.TranscriptDocumentID != nil {
		reused, err := s.reuseTranscriptRecommendation(ctx, userID, *req.TranscriptDocumentID, req.RecommendationPreferenceInput)
		if err == nil {
			return reused, nil
		}
		if !errors.Is(err, errs.ErrSubmissionNotFound) && !errors.Is(err, errs.ErrDocumentNotFound) {
			return CreateRecommendationWorkflowOutput{}, err
		}
		if req.TranscriptFile == nil {
			if errors.Is(err, errs.ErrDocumentNotFound) {
				return CreateRecommendationWorkflowOutput{}, err
			}
			return CreateRecommendationWorkflowOutput{}, errs.ErrExternalService
		}
	}

	return s.processRecommendation(
		ctx,
		userID,
		model.RecommendationModeTranscript,
		&documentUploadRequest{documentType: model.DocumentTypeTranscript, header: req.TranscriptFile},
		nil,
		req.RecommendationPreferenceInput,
	)
}

func (s *RecommendationService) CreateCVRecommendation(
	ctx context.Context,
	userID string,
	req dto.CreateCVRecommendationRequest,
) (CreateRecommendationWorkflowOutput, error) {
	if req.CVFile == nil {
		return CreateRecommendationWorkflowOutput{}, errs.ErrInvalidInput
	}

	return s.processRecommendation(
		ctx,
		userID,
		model.RecommendationModeCV,
		nil,
		&documentUploadRequest{documentType: model.DocumentTypeCV, header: req.CVFile},
		req.RecommendationPreferenceInput,
	)
}

func (s *RecommendationService) GetSubmissionDetail(ctx context.Context, userID, submissionID string) (repository.SubmissionDetail, error) {
	if userID == "" {
		return repository.SubmissionDetail{}, errs.ErrUnauthorized
	}
	return s.repo.FindSubmissionDetail(ctx, submissionID, userID)
}

type documentUploadRequest struct {
	documentType model.DocumentType
	header       *multipart.FileHeader
}

func (s *RecommendationService) processRecommendation(
	ctx context.Context,
	userID string,
	mode model.RecommendationMode,
	transcriptReq *documentUploadRequest,
	cvReq *documentUploadRequest,
	preferences dto.RecommendationPreferenceInput,
) (CreateRecommendationWorkflowOutput, error) {
	if userID == "" {
		return CreateRecommendationWorkflowOutput{}, errs.ErrUnauthorized
	}
	if !s.hasAIClient {
		return CreateRecommendationWorkflowOutput{}, errs.ErrExternalService
	}

	now := time.Now().UTC()
	submissionID := uuid.NewString()
	submittedAt := now

	var transcriptDoc *model.Document
	var cvDoc *model.Document
	var err error

	if transcriptReq != nil {
		transcriptDoc, err = s.uploadAndPersistDocument(ctx, userID, transcriptReq.documentType, transcriptReq.header)
		if err != nil {
			return CreateRecommendationWorkflowOutput{}, err
		}
	}
	if cvReq != nil {
		cvDoc, err = s.uploadAndPersistDocument(ctx, userID, cvReq.documentType, cvReq.header)
		if err != nil {
			return CreateRecommendationWorkflowOutput{}, err
		}
	}

	legacyPrefs := flattenStructuredPreferences(submissionID, now, preferences)
	submission := model.RecommendationSubmission{
		RecSubmissionID:      submissionID,
		UserID:               userID,
		Mode:                 mode,
		TranscriptDocumentID: documentIDPtr(transcriptDoc),
		CVDocumentID:         documentIDPtr(cvDoc),
		Status:               model.RecommendationStatusProcessing,
		CreatedAt:            now,
		SubmittedAt:          &submittedAt,
		UpdatedAt:            now,
	}

	if _, err := s.repo.CreateSubmission(ctx, repository.CreateSubmissionParams{
		Submission:  submission,
		Preferences: legacyPrefs,
	}); err != nil {
		return CreateRecommendationWorkflowOutput{}, err
	}

	allowedCandidates, err := s.buildAllowedCandidates(ctx, preferences)
	if err != nil {
		_ = s.repo.UpdateSubmissionStatus(ctx, submissionID, userID, model.RecommendationStatusFailed)
		return CreateRecommendationWorkflowOutput{}, err
	}

	aiResponse, err := s.callRecommendationAI(ctx, mode, transcriptReq, cvReq, preferences, allowedCandidates)
	if err != nil {
		_ = s.repo.UpdateSubmissionStatus(ctx, submissionID, userID, model.RecommendationStatusFailed)
		return CreateRecommendationWorkflowOutput{}, fmt.Errorf("%w: %v", errs.ErrExternalService, err)
	}

	filteredResponse, err := s.filterRecommendationsToCatalog(ctx, aiResponse, preferences)
	if err != nil {
		_ = s.repo.UpdateSubmissionStatus(ctx, submissionID, userID, model.RecommendationStatusFailed)
		return CreateRecommendationWorkflowOutput{}, err
	}

	filteredResponse, err = s.enrichScholarshipRecommendations(ctx, filteredResponse)
	if err != nil {
		_ = s.repo.UpdateSubmissionStatus(ctx, submissionID, userID, model.RecommendationStatusFailed)
		return CreateRecommendationWorkflowOutput{}, err
	}

	resultSetID := uuid.NewString()
	resultRows, err := mapAIResultsToRows(resultSetID, now, filteredResponse)
	if err != nil {
		_ = s.repo.UpdateSubmissionStatus(ctx, submissionID, userID, model.RecommendationStatusFailed)
		return CreateRecommendationWorkflowOutput{}, fmt.Errorf("map AI result: %w", err)
	}

	if len(resultRows) > 0 {
		if _, err := s.repo.CreateResultSet(ctx, submissionID, now, resultRows); err != nil {
			_ = s.repo.UpdateSubmissionStatus(ctx, submissionID, userID, model.RecommendationStatusFailed)
			return CreateRecommendationWorkflowOutput{}, err
		}
	}

	if err := s.repo.UpdateSubmissionStatus(ctx, submissionID, userID, model.RecommendationStatusCompleted); err != nil {
		return CreateRecommendationWorkflowOutput{}, err
	}

	return CreateRecommendationWorkflowOutput{
		SubmissionID: submissionID,
		Status:       model.RecommendationStatusCompleted,
		ResultSetID:  resultSetID,
		Result:       filteredResponse,
	}, nil
}

func (s *RecommendationService) uploadAndPersistDocument(
	ctx context.Context,
	userID string,
	documentType model.DocumentType,
	header *multipart.FileHeader,
) (*model.Document, error) {
	if err := validateUploadHeader(header); err != nil {
		return nil, err
	}

	stored, err := s.storage.Upload(ctx, UploadInput{
		UserID:       userID,
		DocumentType: documentType,
		Header:       header,
	})
	if err != nil {
		return nil, err
	}

	doc := model.Document{
		DocumentID:       uuid.NewString(),
		UserID:           userID,
		OriginalFilename: header.Filename,
		StoragePath:      stored.StoragePath,
		PublicURL:        stored.PublicURL,
		MIMEType:         stored.MIMEType,
		SizeBytes:        stored.SizeBytes,
		DocumentType:     documentType,
		UploadedAt:       time.Now().UTC(),
	}

	created, err := s.repo.CreateDocument(ctx, doc)
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (s *RecommendationService) callRecommendationAI(
	ctx context.Context,
	mode model.RecommendationMode,
	transcriptReq *documentUploadRequest,
	cvReq *documentUploadRequest,
	preferences dto.RecommendationPreferenceInput,
	allowedCandidates []dto.AIAllowedCandidate,
) (*dto.GlobalMatchAIRecommendationResponse, error) {
	switch mode {
	case model.RecommendationModeProfile:
		resp, err := s.aiClient.RecommendProfile(ctx, dto.AIProfileRecommendationRequest{
			TranscriptFile:    transcriptReq.header,
			CVFile:            cvReq.header,
			Preferences:       preferences,
			AllowedCandidates: allowedCandidates,
		})
		if err != nil {
			return nil, err
		}
		return &resp, nil
	case model.RecommendationModeTranscript:
		resp, err := s.aiClient.RecommendTranscript(ctx, dto.AITranscriptRecommendationRequest{
			TranscriptFile:    transcriptReq.header,
			Preferences:       preferences,
			AllowedCandidates: allowedCandidates,
		})
		if err != nil {
			return nil, err
		}
		return &resp, nil
	case model.RecommendationModeCV:
		resp, err := s.aiClient.RecommendCV(ctx, dto.AICVRecommendationRequest{
			CVFile:            cvReq.header,
			Preferences:       preferences,
			AllowedCandidates: allowedCandidates,
		})
		if err != nil {
			return nil, err
		}
		return &resp, nil
	default:
		return nil, errs.ErrInvalidInput
	}
}

func (s *RecommendationService) buildAllowedCandidates(
	ctx context.Context,
	preferences dto.RecommendationPreferenceInput,
) ([]dto.AIAllowedCandidate, error) {
	countryCodes, err := s.repo.ListRecommendationCountryCodes(ctx)
	if err != nil {
		return nil, err
	}

	preferredCountryCodes := matchPreferredCountryCodes(preferences.Countries, countryCodes)
	candidates, err := s.repo.FindRecommendationAllowedCandidates(ctx, preferredCountryCodes, recommendationAllowedCandidateLimit)
	if err != nil {
		return nil, err
	}

	allowedCandidates := make([]dto.AIAllowedCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.ProgramID) == "" {
			continue
		}
		allowedCandidates = append(allowedCandidates, dto.AIAllowedCandidate{
			ProgramID:             candidate.ProgramID,
			ProgramName:           candidate.ProgramName,
			UniversityName:        candidate.UniversityName,
			Country:               countryCodeDisplayName(candidate.CountryCode),
			OfficialProgramURL:    strings.TrimSpace(candidate.OfficialProgramURL),
			OfficialUniversityURL: strings.TrimSpace(candidate.OfficialUniversityURL),
		})
	}

	return allowedCandidates, nil
}

func (s *RecommendationService) reuseTranscriptRecommendation(
	ctx context.Context,
	userID string,
	documentID string,
	preferences dto.RecommendationPreferenceInput,
) (CreateRecommendationWorkflowOutput, error) {
	if userID == "" {
		return CreateRecommendationWorkflowOutput{}, errs.ErrUnauthorized
	}

	doc, err := s.repo.FindDocumentByIDAndUser(ctx, documentID, userID)
	if err != nil {
		return CreateRecommendationWorkflowOutput{}, err
	}
	if doc.DocumentType != model.DocumentTypeTranscript {
		return CreateRecommendationWorkflowOutput{}, errs.ErrInvalidInput
	}

	detail, err := s.repo.FindLatestCompletedSubmissionByTranscriptDocument(ctx, userID, documentID)
	if err != nil {
		return CreateRecommendationWorkflowOutput{}, err
	}

	requestPrefs := flattenStructuredPreferences("compare", time.Time{}, preferences)
	if !sameRecommendationPreferences(detail.Preferences, requestPrefs) {
		return CreateRecommendationWorkflowOutput{}, errs.ErrSubmissionNotFound
	}
	if detail.LatestResultSet == nil {
		return CreateRecommendationWorkflowOutput{}, errs.ErrSubmissionNotFound
	}

	return CreateRecommendationWorkflowOutput{
		SubmissionID: detail.Submission.RecSubmissionID,
		Status:       detail.Submission.Status,
		ResultSetID:  detail.LatestResultSet.ResultSetID,
		Result:       submissionDetailToAIResponse(detail),
	}, nil
}

func sameRecommendationPreferences(existing []model.RecommendationPreference, requested []model.RecommendationPreference) bool {
	if len(existing) != len(requested) {
		return false
	}

	counts := make(map[string]int, len(existing))
	for _, pref := range existing {
		key := pref.PreferenceKey + "\x00" + pref.PreferenceValue
		counts[key]++
	}
	for _, pref := range requested {
		key := pref.PreferenceKey + "\x00" + pref.PreferenceValue
		counts[key]--
		if counts[key] < 0 {
			return false
		}
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

func submissionDetailToAIResponse(detail repository.SubmissionDetail) *dto.GlobalMatchAIRecommendationResponse {
	if detail.LatestResultSet == nil {
		return nil
	}

	topRecommendations := make([]dto.GlobalMatchAITopRecommendationResponse, 0, len(detail.Results))
	for _, row := range detail.Results {
		pros := make([]string, 0)
		cons := make([]string, 0)
		preferenceReasoning := make([]string, 0)
		matchEvidence := make([]string, 0)
		scholarships := make([]dto.GlobalMatchAIScholarshipRecommendationResponse, 0)

		if err := json.Unmarshal([]byte(row.ProsJSON), &pros); err != nil {
			pros = []string{}
		}
		if err := json.Unmarshal([]byte(row.ConsJSON), &cons); err != nil {
			cons = []string{}
		}
		if row.PreferenceReasoningJSON != "" {
			if err := json.Unmarshal([]byte(row.PreferenceReasoningJSON), &preferenceReasoning); err != nil {
				preferenceReasoning = []string{}
			}
		}
		if row.MatchEvidenceJSON != "" {
			if err := json.Unmarshal([]byte(row.MatchEvidenceJSON), &matchEvidence); err != nil {
				matchEvidence = []string{}
			}
		}
		if row.ScholarshipRecommendationsJSON != "" {
			if err := json.Unmarshal([]byte(row.ScholarshipRecommendationsJSON), &scholarships); err != nil {
				scholarships = []dto.GlobalMatchAIScholarshipRecommendationResponse{}
			}
		}
		if len(preferenceReasoning) == 0 && row.ReasonSummary != "" {
			preferenceReasoning = []string{row.ReasonSummary}
		}
		if len(matchEvidence) == 0 && row.ReasonSummary != "" {
			matchEvidence = []string{row.ReasonSummary}
		}

		topRecommendations = append(topRecommendations, dto.GlobalMatchAITopRecommendationResponse{
			Rank:                       row.RankNo,
			ProgramID:                  row.ProgramID,
			UniversityName:             row.UniversityName,
			ProgramName:                row.ProgramName,
			Country:                    row.Country,
			FitScore:                   row.FitScore,
			AdmissionChanceScore:       row.AdmissionChanceScore,
			OverallRecommendationScore: row.OverallRecommendationScore,
			FitLevel:                   row.FitLevel,
			AdmissionDifficulty:        row.AdmissionDifficulty,
			Overview:                   row.Overview,
			WhyThisUniversity:          row.WhyThisUniversity,
			WhyThisProgram:             row.WhyThisProgram,
			PreferenceReasoning:        preferenceReasoning,
			MatchEvidence:              matchEvidence,
			ScholarshipRecommendations: scholarships,
			Pros:                       pros,
			Cons:                       cons,
		})
	}

	return &dto.GlobalMatchAIRecommendationResponse{
		TopRecommendations: topRecommendations,
	}
}

func mapAIResultsToRows(
	resultSetID string,
	now time.Time,
	resp *dto.GlobalMatchAIRecommendationResponse,
) ([]model.RecommendationResult, error) {
	if resp == nil {
		return nil, errs.ErrInvalidInput
	}

	rows := make([]model.RecommendationResult, 0, len(resp.TopRecommendations))
	for _, item := range resp.TopRecommendations {
		scoreBreakdownJSON, err := json.Marshal(item.ScoreBreakdown)
		if err != nil {
			return nil, err
		}
		preferenceReasoningJSON, err := json.Marshal(item.PreferenceReasoning)
		if err != nil {
			return nil, err
		}
		matchEvidenceJSON, err := json.Marshal(item.MatchEvidence)
		if err != nil {
			return nil, err
		}
		scholarshipJSON, err := json.Marshal(item.ScholarshipRecommendations)
		if err != nil {
			return nil, err
		}
		prosJSON, err := json.Marshal(item.Pros)
		if err != nil {
			return nil, err
		}
		consJSON, err := json.Marshal(item.Cons)
		if err != nil {
			return nil, err
		}
		rawJSON, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}

		rows = append(rows, model.RecommendationResult{
			RecResultID:                    uuid.NewString(),
			ResultSetID:                    resultSetID,
			ProgramID:                      item.ProgramID,
			RankNo:                         item.Rank,
			UniversityName:                 item.UniversityName,
			ProgramName:                    item.ProgramName,
			Country:                        item.Country,
			FitScore:                       item.FitScore,
			AdmissionChanceScore:           item.AdmissionChanceScore,
			OverallRecommendationScore:     item.OverallRecommendationScore,
			FitLevel:                       item.FitLevel,
			AdmissionDifficulty:            item.AdmissionDifficulty,
			ScoreBreakdownJSON:             string(scoreBreakdownJSON),
			Overview:                       item.Overview,
			WhyThisUniversity:              item.WhyThisUniversity,
			WhyThisProgram:                 item.WhyThisProgram,
			PreferenceReasoningJSON:        string(preferenceReasoningJSON),
			MatchEvidenceJSON:              string(matchEvidenceJSON),
			ScholarshipRecommendationsJSON: string(scholarshipJSON),
			ReasonSummary:                  resp.SelectionReasoning,
			ProsJSON:                       string(prosJSON),
			ConsJSON:                       string(consJSON),
			RawRecommendationJSON:          string(rawJSON),
			CreatedAt:                      now,
		})
	}

	return rows, nil
}

func (s *RecommendationService) filterRecommendationsToCatalog(
	ctx context.Context,
	resp *dto.GlobalMatchAIRecommendationResponse,
	preferences dto.RecommendationPreferenceInput,
) (*dto.GlobalMatchAIRecommendationResponse, error) {
	if resp == nil {
		return nil, errs.ErrInvalidInput
	}

	lookups := make([]repository.RecommendationProgramLookup, 0, len(resp.TopRecommendations))
	for _, item := range resp.TopRecommendations {
		if item.ProgramID != nil && strings.TrimSpace(*item.ProgramID) != "" {
			continue
		}
		lookups = append(lookups, repository.RecommendationProgramLookup{
			UniversityName: item.UniversityName,
			ProgramName:    item.ProgramName,
		})
	}

	matchedProgramIDs := make(map[repository.RecommendationProgramLookup]string)
	if len(lookups) > 0 {
		matches, err := s.repo.FindMatchingPrograms(ctx, lookups)
		if err != nil {
			return nil, err
		}

		matchedProgramIDs = make(map[repository.RecommendationProgramLookup]string, len(matches))
		for _, match := range matches {
			key := repository.RecommendationProgramLookup{
				UniversityName: normalizeRecommendationCatalogValue(match.UniversityName),
				ProgramName:    normalizeRecommendationCatalogValue(match.ProgramName),
			}
			matchedProgramIDs[key] = match.ProgramID
		}
	}

	filtered := *resp
	filtered.TopRecommendations = make([]dto.GlobalMatchAITopRecommendationResponse, 0, len(resp.TopRecommendations))
	for _, item := range resp.TopRecommendations {
		if item.ProgramID != nil && strings.TrimSpace(*item.ProgramID) != "" {
			filtered.TopRecommendations = append(filtered.TopRecommendations, item)
			continue
		}

		key := repository.RecommendationProgramLookup{
			UniversityName: normalizeRecommendationCatalogValue(item.UniversityName),
			ProgramName:    normalizeRecommendationCatalogValue(item.ProgramName),
		}
		programID, ok := matchedProgramIDs[key]
		if !ok {
			continue
		}

		cloned := item
		cloned.ProgramID = &programID
		filtered.TopRecommendations = append(filtered.TopRecommendations, cloned)
	}

	if len(filtered.TopRecommendations) == 0 {
		return nil, errs.ErrExternalService
	}

	filtered.TopRecommendations = prioritizeRecommendations(filtered.TopRecommendations, preferences.Countries)
	for idx := range filtered.TopRecommendations {
		filtered.TopRecommendations[idx].Rank = idx + 1
	}

	return &filtered, nil
}

func prioritizeRecommendations(
	recommendations []dto.GlobalMatchAITopRecommendationResponse,
	preferredCountries []string,
) []dto.GlobalMatchAITopRecommendationResponse {
	if len(recommendations) <= 1 {
		return recommendations
	}

	preferredCountrySet := make(map[string]struct{}, len(preferredCountries))
	for _, country := range preferredCountries {
		normalized := normalizeRecommendationCatalogValue(country)
		if normalized == "" {
			continue
		}
		preferredCountrySet[normalized] = struct{}{}
	}
	if len(preferredCountrySet) == 0 {
		return recommendations
	}

	preferred := make([]dto.GlobalMatchAITopRecommendationResponse, 0, len(recommendations))
	others := make([]dto.GlobalMatchAITopRecommendationResponse, 0, len(recommendations))
	for _, item := range recommendations {
		if _, ok := preferredCountrySet[normalizeRecommendationCatalogValue(item.Country)]; ok {
			preferred = append(preferred, item)
			continue
		}
		others = append(others, item)
	}

	switch len(preferred) {
	case 0:
		return recommendations
	case 1:
		ordered := make([]dto.GlobalMatchAITopRecommendationResponse, 0, len(recommendations))
		ordered = append(ordered, preferred[0])
		ordered = append(ordered, others...)
		return ordered
	default:
		ordered := make([]dto.GlobalMatchAITopRecommendationResponse, 0, len(recommendations))
		ordered = append(ordered, preferred...)
		ordered = append(ordered, others...)
		return ordered
	}
}

func normalizeRecommendationCatalogValue(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(strings.ToLower(value))), " ")
}

func (s *RecommendationService) enrichScholarshipRecommendations(
	ctx context.Context,
	resp *dto.GlobalMatchAIRecommendationResponse,
) (*dto.GlobalMatchAIRecommendationResponse, error) {
	if resp == nil {
		return nil, errs.ErrInvalidInput
	}

	programIDs := make([]string, 0, len(resp.TopRecommendations))
	seenProgramIDs := make(map[string]struct{}, len(resp.TopRecommendations))
	for _, item := range resp.TopRecommendations {
		if item.ProgramID == nil || strings.TrimSpace(*item.ProgramID) == "" {
			continue
		}
		if _, ok := seenProgramIDs[*item.ProgramID]; ok {
			continue
		}
		seenProgramIDs[*item.ProgramID] = struct{}{}
		programIDs = append(programIDs, *item.ProgramID)
	}
	if len(programIDs) == 0 {
		return resp, nil
	}

	matches, err := s.repo.FindScholarshipMatches(ctx, programIDs)
	if err != nil {
		return nil, err
	}

	type scholarshipLink struct {
		fundingID   string
		admissionID string
	}
	type scholarshipMatchAggregate struct {
		link          scholarshipLink
		fundingIDs    map[string]struct{}
		admissionSeen map[string]struct{}
	}
	matchIndex := make(map[string]*scholarshipMatchAggregate, len(matches))
	for _, match := range matches {
		key := scholarshipMatchKey(match.ProgramID, match.ScholarshipName)
		aggregate, ok := matchIndex[key]
		if !ok {
			matchIndex[key] = &scholarshipMatchAggregate{
				link: scholarshipLink{
					fundingID:   match.FundingID,
					admissionID: match.AdmissionID,
				},
				fundingIDs: map[string]struct{}{
					match.FundingID: {},
				},
				admissionSeen: map[string]struct{}{},
			}
			if strings.TrimSpace(match.AdmissionID) != "" {
				matchIndex[key].admissionSeen[match.AdmissionID] = struct{}{}
			}
			continue
		}
		if strings.TrimSpace(match.FundingID) != "" {
			aggregate.fundingIDs[match.FundingID] = struct{}{}
		}
		if strings.TrimSpace(aggregate.link.admissionID) == "" && strings.TrimSpace(match.AdmissionID) != "" {
			aggregate.link.admissionID = match.AdmissionID
		}
		if strings.TrimSpace(match.AdmissionID) != "" {
			aggregate.admissionSeen[match.AdmissionID] = struct{}{}
		}
	}

	clone := *resp
	clone.TopRecommendations = make([]dto.GlobalMatchAITopRecommendationResponse, 0, len(resp.TopRecommendations))
	for _, item := range resp.TopRecommendations {
		copied := item
		copied.ScholarshipRecommendations = append([]dto.GlobalMatchAIScholarshipRecommendationResponse(nil), item.ScholarshipRecommendations...)
		if copied.ProgramID != nil && strings.TrimSpace(*copied.ProgramID) != "" {
			for idx := range copied.ScholarshipRecommendations {
				key := scholarshipMatchKey(*copied.ProgramID, copied.ScholarshipRecommendations[idx].ScholarshipName)
				aggregate, ok := matchIndex[key]
				if !ok || len(aggregate.fundingIDs) != 1 {
					continue
				}
				copied.ScholarshipRecommendations[idx].FundingID = optionalStringPtr(aggregate.link.fundingID)
				copied.ScholarshipRecommendations[idx].AdmissionID = optionalStringPtr(aggregate.link.admissionID)
			}
		}
		clone.TopRecommendations = append(clone.TopRecommendations, copied)
	}

	return &clone, nil
}

func scholarshipMatchKey(programID, scholarshipName string) string {
	return strings.TrimSpace(programID) + "\x00" + normalizeRecommendationCatalogValue(scholarshipName)
}

func optionalStringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func matchPreferredCountryCodes(preferredCountries []string, countryCodes []string) []string {
	if len(preferredCountries) == 0 || len(countryCodes) == 0 {
		return nil
	}

	preferred := make(map[string]struct{}, len(preferredCountries))
	for _, country := range preferredCountries {
		normalized := normalizeRecommendationCatalogValue(country)
		if normalized == "" {
			continue
		}
		preferred[normalized] = struct{}{}
	}
	if len(preferred) == 0 {
		return nil
	}

	matchedCodes := make([]string, 0, len(countryCodes))
	for _, code := range countryCodes {
		normalizedCode := normalizeRecommendationCatalogValue(code)
		normalizedName := normalizeRecommendationCatalogValue(countryCodeDisplayName(code))
		if _, ok := preferred[normalizedCode]; ok {
			matchedCodes = append(matchedCodes, code)
			continue
		}
		if _, ok := preferred[normalizedName]; ok {
			matchedCodes = append(matchedCodes, code)
		}
	}

	return matchedCodes
}

func countryCodeDisplayName(code string) string {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return ""
	}
	region, err := language.ParseRegion(trimmed)
	if err != nil {
		return trimmed
	}
	name := display.English.Regions().Name(region)
	if strings.TrimSpace(name) == "" {
		return strings.ToUpper(trimmed)
	}
	return name
}

func buildLegacyPreferences(submissionID string, now time.Time, preferences []dto.PreferenceInput) ([]model.RecommendationPreference, error) {
	result := make([]model.RecommendationPreference, 0, len(preferences))
	for _, pref := range preferences {
		if strings.TrimSpace(pref.Key) == "" || strings.TrimSpace(pref.Value) == "" {
			return nil, errs.ErrInvalidInput
		}
		result = append(result, model.RecommendationPreference{
			PrefID:          uuid.NewString(),
			RecSubmissionID: submissionID,
			PreferenceKey:   pref.Key,
			PreferenceValue: pref.Value,
			CreatedAt:       now,
		})
	}
	return result, nil
}

func flattenStructuredPreferences(
	submissionID string,
	now time.Time,
	preferences dto.RecommendationPreferenceInput,
) []model.RecommendationPreference {
	result := make([]model.RecommendationPreference, 0, 16)
	addStringSlicePreference(&result, submissionID, now, "continents", preferences.Continents)
	addStringSlicePreference(&result, submissionID, now, "countries", preferences.Countries)
	addStringSlicePreference(&result, submissionID, now, "fields_of_study", preferences.FieldsOfStudy)
	addStringPreference(&result, submissionID, now, "degree_level", preferences.DegreeLevel)
	addStringSlicePreference(&result, submissionID, now, "languages", preferences.Languages)
	addStringSlicePreference(&result, submissionID, now, "budget_preferences", preferences.BudgetPreferences)
	addStringSlicePreference(&result, submissionID, now, "scholarship_types", preferences.ScholarshipTypes)
	addStringSlicePreference(&result, submissionID, now, "start_periods", preferences.StartPeriods)
	addStringPreference(&result, submissionID, now, "additional_preference", preferences.AdditionalPreference)
	return result
}

func addStringSlicePreference(target *[]model.RecommendationPreference, submissionID string, now time.Time, key string, values []string) {
	for _, value := range values {
		addStringPreference(target, submissionID, now, key, value)
	}
}

func addStringPreference(target *[]model.RecommendationPreference, submissionID string, now time.Time, key, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	*target = append(*target, model.RecommendationPreference{
		PrefID:          uuid.NewString(),
		RecSubmissionID: submissionID,
		PreferenceKey:   key,
		PreferenceValue: trimmed,
		CreatedAt:       now,
	})
}

func documentIDPtr(doc *model.Document) *string {
	if doc == nil {
		return nil
	}
	return &doc.DocumentID
}

func validateUploadHeader(header *multipart.FileHeader) error {
	if header == nil {
		return errs.ErrInvalidInput
	}
	if header.Size <= 0 || header.Size > MaxDocumentSizeBytes {
		return errs.ErrInvalidInput
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if _, ok := allowedDocumentExtensions[ext]; !ok {
		return errs.ErrInvalidInput
	}

	return nil
}

func detectContentType(data []byte) string {
	contentType := http.DetectContentType(data)
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}
