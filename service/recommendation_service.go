package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"

	"github.com/google/uuid"
)

const (
	MaxDocumentSizeBytes int64 = 5 * 1024 * 1024
)

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

type DocumentUploader interface {
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
	storage     DocumentUploader
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
	storage DocumentUploader,
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
	if req.TranscriptFile == nil {
		return CreateRecommendationWorkflowOutput{}, errs.ErrInvalidInput
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

func (s *RecommendationService) PreviewAllowedCandidates(
	ctx context.Context,
	userID string,
	preferences dto.RecommendationPreferenceInput,
) ([]dto.RecommendationAllowedCandidateInput, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errs.ErrUnauthorized
	}
	return s.buildAllowedCandidates(ctx, flattenStructuredPreferences(uuid.NewString(), time.Now().UTC(), preferences))
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
	log.Printf("recommendation_service_started mode=%s user_id=%s has_transcript=%t has_cv=%t", mode, userID, transcriptReq != nil, cvReq != nil)

	now := time.Now().UTC()
	submissionID := uuid.NewString()
	submittedAt := now

	var transcriptDoc *model.Document
	var cvDoc *model.Document
	var err error

	if transcriptReq != nil {
		transcriptDoc, err = s.uploadAndPersistDocument(ctx, userID, transcriptReq.documentType, transcriptReq.header)
		if err != nil {
			log.Printf("recommendation_service_document_error mode=%s stage=transcript_upload err=%v", mode, err)
			return CreateRecommendationWorkflowOutput{}, err
		}
	}
	if cvReq != nil {
		cvDoc, err = s.uploadAndPersistDocument(ctx, userID, cvReq.documentType, cvReq.header)
		if err != nil {
			log.Printf("recommendation_service_document_error mode=%s stage=cv_upload err=%v", mode, err)
			return CreateRecommendationWorkflowOutput{}, err
		}
	}

	legacyPrefs := flattenStructuredPreferences(submissionID, now, preferences)
	allowedCandidates, err := s.buildAllowedCandidates(ctx, legacyPrefs)
	if err != nil {
		log.Printf("recommendation_service_candidates_error mode=%s submission_id=%s err=%v", mode, submissionID, err)
		return CreateRecommendationWorkflowOutput{}, err
	}
	log.Printf("recommendation_service_candidates_built mode=%s submission_id=%s count=%d", mode, submissionID, len(allowedCandidates))
	if len(allowedCandidates) == 0 {
		return CreateRecommendationWorkflowOutput{}, errs.ErrExternalService
	}
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
		log.Printf("recommendation_service_submission_error mode=%s submission_id=%s err=%v", mode, submissionID, err)
		return CreateRecommendationWorkflowOutput{}, err
	}
	log.Printf("recommendation_service_submission_created mode=%s submission_id=%s", mode, submissionID)

	log.Printf("recommendation_service_call_ai mode=%s submission_id=%s candidates=%d", mode, submissionID, len(allowedCandidates))
	aiResponse, err := s.callRecommendationAI(ctx, mode, transcriptReq, cvReq, preferences, allowedCandidates)
	if err != nil {
		_ = s.repo.UpdateSubmissionStatus(ctx, submissionID, userID, model.RecommendationStatusFailed)
		log.Printf("recommendation_service_ai_error mode=%s submission_id=%s err=%v", mode, submissionID, err)
		return CreateRecommendationWorkflowOutput{}, fmt.Errorf("%w: %v", errs.ErrExternalService, err)
	}
	log.Printf("recommendation_service_ai_success mode=%s submission_id=%s top_recommendations=%d", mode, submissionID, len(aiResponse.TopRecommendations))

	resultSetID := uuid.NewString()
	resultRows, err := mapAIResultsToRows(resultSetID, now, aiResponse)
	if err != nil {
		_ = s.repo.UpdateSubmissionStatus(ctx, submissionID, userID, model.RecommendationStatusFailed)
		log.Printf("recommendation_service_map_error mode=%s submission_id=%s err=%v", mode, submissionID, err)
		return CreateRecommendationWorkflowOutput{}, fmt.Errorf("map AI result: %w", err)
	}

	if len(resultRows) > 0 {
		if _, err := s.repo.CreateResultSet(ctx, submissionID, now, resultRows); err != nil {
			_ = s.repo.UpdateSubmissionStatus(ctx, submissionID, userID, model.RecommendationStatusFailed)
			log.Printf("recommendation_service_resultset_error mode=%s submission_id=%s err=%v", mode, submissionID, err)
			return CreateRecommendationWorkflowOutput{}, err
		}
	}

	if err := s.repo.UpdateSubmissionStatus(ctx, submissionID, userID, model.RecommendationStatusCompleted); err != nil {
		log.Printf("recommendation_service_status_error mode=%s submission_id=%s err=%v", mode, submissionID, err)
		return CreateRecommendationWorkflowOutput{}, err
	}
	log.Printf("recommendation_service_completed mode=%s submission_id=%s result_set_id=%s", mode, submissionID, resultSetID)

	return CreateRecommendationWorkflowOutput{
		SubmissionID: submissionID,
		Status:       model.RecommendationStatusCompleted,
		ResultSetID:  resultSetID,
		Result:       aiResponse,
	}, nil
}

func (s *RecommendationService) buildAllowedCandidates(
	ctx context.Context,
	preferences []model.RecommendationPreference,
) ([]dto.RecommendationAllowedCandidateInput, error) {
	log.Printf("recommendation_service_candidates_filters %s", summarizeRecommendationPreferences(preferences))
	items, err := s.repo.ListRecommendationCandidates(ctx, preferences)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		relaxed := relaxRecommendationPreferences(preferences)
		log.Printf("recommendation_service_candidates_retry stage=relaxed filters=%s", summarizeRecommendationPreferences(relaxed))
		items, err = s.repo.ListRecommendationCandidates(ctx, relaxed)
		if err != nil {
			return nil, err
		}
	}
	if len(items) == 0 {
		coreOnly := keepOnlyCoreRecommendationPreferences(preferences)
		log.Printf("recommendation_service_candidates_retry stage=core_only filters=%s", summarizeRecommendationPreferences(coreOnly))
		items, err = s.repo.ListRecommendationCandidates(ctx, coreOnly)
		if err != nil {
			return nil, err
		}
	}

	candidates := make([]dto.RecommendationAllowedCandidateInput, 0, len(items))
	for _, item := range items {
		candidates = append(candidates, dto.RecommendationAllowedCandidateInput{
			ProgramID:             item.ProgramID,
			ProgramName:           item.ProgramName,
			UniversityName:        item.UniversityName,
			Country:               item.Country,
			DegreeLevel:           item.DegreeLevel,
			Language:              item.Language,
			FocusTags:             buildFocusTags(item.ProgramName),
			FundingSummary:        splitDelimitedValues(item.FundingSummary, "||"),
			AdmissionDeadline:     item.AdmissionDeadline,
			OfficialProgramURL:    item.OfficialProgramURL,
			OfficialUniversityURL: item.OfficialUniversityURL,
		})
	}
	return candidates, nil
}

func summarizeRecommendationPreferences(preferences []model.RecommendationPreference) string {
	parts := make([]string, 0, len(preferences))
	for _, pref := range preferences {
		key := strings.TrimSpace(pref.PreferenceKey)
		value := strings.TrimSpace(pref.PreferenceValue)
		if key == "" || value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, "; ")
}

func relaxRecommendationPreferences(preferences []model.RecommendationPreference) []model.RecommendationPreference {
	result := make([]model.RecommendationPreference, 0, len(preferences))
	for _, pref := range preferences {
		key := strings.ToLower(strings.TrimSpace(pref.PreferenceKey))
		switch key {
		case "fields_of_study", "field_of_study", "field", "start_periods", "start_period", "scholarship_types", "scholarship_type", "additional_preference":
			continue
		default:
			result = append(result, pref)
		}
	}
	return result
}

func keepOnlyCoreRecommendationPreferences(preferences []model.RecommendationPreference) []model.RecommendationPreference {
	result := make([]model.RecommendationPreference, 0, len(preferences))
	for _, pref := range preferences {
		key := strings.ToLower(strings.TrimSpace(pref.PreferenceKey))
		switch key {
		case "continents", "continent", "countries", "country", "degree_level":
			result = append(result, pref)
		}
	}
	return result
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
	allowedCandidates []dto.RecommendationAllowedCandidateInput,
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
		payloads, err := marshalRecommendationPayloads(item)
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
			ScoreBreakdownJSON:             payloads.ScoreBreakdownJSON,
			Overview:                       item.Overview,
			WhyThisUniversity:              item.WhyThisUniversity,
			WhyThisProgram:                 item.WhyThisProgram,
			PreferenceReasoningJSON:        payloads.PreferenceReasoningJSON,
			MatchEvidenceJSON:              payloads.MatchEvidenceJSON,
			ScholarshipRecommendationsJSON: payloads.ScholarshipRecommendationsJSON,
			ReasonSummary:                  resp.SelectionReasoning,
			ProsJSON:                       payloads.ProsJSON,
			ConsJSON:                       payloads.ConsJSON,
			RawRecommendationJSON:          payloads.RawRecommendationJSON,
			CreatedAt:                      now,
		})
	}

	return rows, nil
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

func splitDelimitedValues(input, delimiter string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	parts := strings.Split(input, delimiter)
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func buildFocusTags(programName string) []string {
	normalized := strings.NewReplacer("/", " ", "-", " ", ",", " ", "(", " ", ")", " ").Replace(strings.ToLower(programName))
	words := strings.Fields(normalized)
	if len(words) == 0 {
		return nil
	}
	stopWords := map[string]struct{}{
		"and": {}, "the": {}, "for": {}, "with": {}, "master": {}, "doctor": {}, "doctoral": {},
		"msc": {}, "ma": {}, "mba": {}, "phd": {}, "of": {}, "in": {}, "science": {}, "arts": {},
	}
	result := make([]string, 0, 5)
	seen := make(map[string]struct{}, 5)
	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		if _, ok := stopWords[word]; ok {
			continue
		}
		if _, ok := seen[word]; ok {
			continue
		}
		seen[word] = struct{}{}
		result = append(result, word)
		if len(result) == 5 {
			break
		}
	}
	return result
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

type recommendationPayloads struct {
	ScoreBreakdownJSON             string
	PreferenceReasoningJSON        string
	MatchEvidenceJSON              string
	ScholarshipRecommendationsJSON string
	ProsJSON                       string
	ConsJSON                       string
	RawRecommendationJSON          string
}

func marshalRecommendationPayloads(item dto.GlobalMatchAITopRecommendationResponse) (recommendationPayloads, error) {
	scoreBreakdownJSON, err := json.Marshal(item.ScoreBreakdown)
	if err != nil {
		return recommendationPayloads{}, err
	}
	preferenceReasoningJSON, err := json.Marshal(item.PreferenceReasoning)
	if err != nil {
		return recommendationPayloads{}, err
	}
	matchEvidenceJSON, err := json.Marshal(item.MatchEvidence)
	if err != nil {
		return recommendationPayloads{}, err
	}
	scholarshipJSON, err := json.Marshal(item.ScholarshipRecommendations)
	if err != nil {
		return recommendationPayloads{}, err
	}
	prosJSON, err := json.Marshal(item.Pros)
	if err != nil {
		return recommendationPayloads{}, err
	}
	consJSON, err := json.Marshal(item.Cons)
	if err != nil {
		return recommendationPayloads{}, err
	}
	rawJSON, err := json.Marshal(item)
	if err != nil {
		return recommendationPayloads{}, err
	}

	return recommendationPayloads{
		ScoreBreakdownJSON:             string(scoreBreakdownJSON),
		PreferenceReasoningJSON:        string(preferenceReasoningJSON),
		MatchEvidenceJSON:              string(matchEvidenceJSON),
		ScholarshipRecommendationsJSON: string(scholarshipJSON),
		ProsJSON:                       string(prosJSON),
		ConsJSON:                       string(consJSON),
		RawRecommendationJSON:          string(rawJSON),
	}, nil
}
