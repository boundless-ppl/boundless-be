package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

type DocumentStorage interface {
	Upload(ctx context.Context, input UploadInput) (StoredObject, error)
}

// PAKE LOCAL DULU SAMPAI UDH SETUP STORAGE
type LocalDocumentStorage struct {
	baseDir string
	baseURL string
}

func NewLocalDocumentStorage(baseDir, baseURL string) *LocalDocumentStorage {
	if baseDir == "" {
		baseDir = "uploads"
	}
	return &LocalDocumentStorage{baseDir: baseDir, baseURL: strings.TrimSuffix(baseURL, "/")}
}

func (s *LocalDocumentStorage) Upload(ctx context.Context, input UploadInput) (StoredObject, error) {
	if input.Header == nil {
		return StoredObject{}, errs.ErrInvalidInput
	}

	src, err := input.Header.Open()
	if err != nil {
		return StoredObject{}, fmt.Errorf("open file: %w", err)
	}
	defer src.Close()

	ext := strings.ToLower(filepath.Ext(input.Header.Filename))
	if _, ok := allowedDocumentExtensions[ext]; !ok {
		return StoredObject{}, errs.ErrInvalidInput
	}

	objectName := fmt.Sprintf("%s/%s/%s%s",
		input.UserID,
		input.DocumentType,
		uuid.NewString(),
		ext,
	)

	fullPath := filepath.Join(s.baseDir, objectName)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return StoredObject{}, fmt.Errorf("create directory: %w", err)
	}

	dst, err := os.Create(fullPath)
	if err != nil {
		return StoredObject{}, fmt.Errorf("create file: %w", err)
	}
	defer dst.Close()

	buffer := make([]byte, 512)
	n, err := src.Read(buffer)
	if err != nil && err != io.EOF {
		return StoredObject{}, fmt.Errorf("read file header: %w", err)
	}

	mimeType := http.DetectContentType(buffer[:n])

	if n > 0 {
		if _, err := dst.Write(buffer[:n]); err != nil {
			return StoredObject{}, fmt.Errorf("write file header: %w", err)
		}
	}

	limited := io.LimitReader(src, MaxDocumentSizeBytes+1)

	copied, err := io.Copy(dst, limited)
	if err != nil {
		return StoredObject{}, fmt.Errorf("copy file: %w", err)
	}

	size := int64(n) + copied

	if size > MaxDocumentSizeBytes {
		dst.Close()
		os.Remove(fullPath) // clean up oversized file
		return StoredObject{}, errs.ErrInvalidInput
	}

	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	publicURL := fullPath
	if s.baseURL != "" {
		publicURL = s.baseURL + "/" + filepath.ToSlash(objectName)
	}

	return StoredObject{
		StoragePath: fullPath,
		PublicURL:   publicURL,
		SizeBytes:   size,
		MIMEType:    mimeType,
	}, nil
}

type AIRecommendationClient interface {
	GenerateRecommendations(ctx context.Context, req dto.AIRecommendationRequest) (dto.AIRecommendationResponse, error)
}

type HTTPAIRecommendationClient struct {
	httpClient *http.Client
	endpoint   string
}

func NewHTTPAIRecommendationClient(endpoint string) *HTTPAIRecommendationClient {
	return &HTTPAIRecommendationClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		endpoint:   strings.TrimSuffix(endpoint, "/"),
	}
}

func (c *HTTPAIRecommendationClient) GenerateRecommendations(ctx context.Context, req dto.AIRecommendationRequest) (dto.AIRecommendationResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return dto.AIRecommendationResponse{}, fmt.Errorf("marshal AI request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/recommendations/generate", bytes.NewReader(body))
	if err != nil {
		return dto.AIRecommendationResponse{}, fmt.Errorf("build AI request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return dto.AIRecommendationResponse{}, fmt.Errorf("call AI service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return dto.AIRecommendationResponse{}, fmt.Errorf("AI service status %d", resp.StatusCode)
	}

	var payload dto.AIRecommendationResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return dto.AIRecommendationResponse{}, fmt.Errorf("decode AI response: %w", err)
	}

	if payload.GeneratedAt.IsZero() {
		payload.GeneratedAt = time.Now().UTC()
	}

	return payload, nil
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

type UploadDocumentInput struct {
	UserID       string
	DocumentType model.DocumentType
	File         *multipart.FileHeader
}

type UploadDocumentOutput struct {
	Document model.Document
}

type RecommendationService struct {
	repo        repository.RecommendationRepository
	storage     DocumentStorage
	aiClient    AIRecommendationClient
	hasAIClient bool
}

func NewRecommendationService(repo repository.RecommendationRepository) *RecommendationService {
	storage := NewLocalDocumentStorage(os.Getenv("DOCUMENT_STORAGE_DIR"), os.Getenv("DOCUMENT_PUBLIC_BASE_URL"))

	aiURL := strings.TrimSpace(os.Getenv("AI_SERVICE_URL"))
	service := &RecommendationService{
		repo:    repo,
		storage: storage,
	}
	if aiURL != "" {
		service.aiClient = NewHTTPAIRecommendationClient(aiURL)
		service.hasAIClient = true
	}
	return service
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
	status := model.RecommendationStatusDraft
	submittedAt := (*time.Time)(nil)
	if s.hasAIClient {
		status = model.RecommendationStatusProcessing
		submittedAt = &now
	}

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

	preferences := make([]model.RecommendationPreference, 0, len(input.Preferences))
	aiPrefs := make([]dto.AIPreference, 0, len(input.Preferences))
	for _, pref := range input.Preferences {
		if strings.TrimSpace(pref.Key) == "" || strings.TrimSpace(pref.Value) == "" {
			return CreateSubmissionOutput{}, errs.ErrInvalidInput
		}

		preferences = append(preferences, model.RecommendationPreference{
			PrefID:          uuid.NewString(),
			RecSubmissionID: submissionID,
			PreferenceKey:   pref.Key,
			PreferenceValue: pref.Value,
			CreatedAt:       now,
		})
		aiPrefs = append(aiPrefs, dto.AIPreference{Key: pref.Key, Value: pref.Value})
	}

	var transcriptID *string
	if transcriptDoc != nil {
		transcriptID = &transcriptDoc.DocumentID
	}
	var cvID *string
	if cvDoc != nil {
		cvID = &cvDoc.DocumentID
	}

	submission := model.RecommendationSubmission{
		RecSubmissionID:      submissionID,
		UserID:               input.UserID,
		TranscriptDocumentID: transcriptID,
		CVDocumentID:         cvID,
		Status:               status,
		CreatedAt:            now,
		SubmittedAt:          submittedAt,
	}

	_, err := s.repo.CreateSubmission(ctx, repository.CreateSubmissionParams{
		Submission:  submission,
		Preferences: preferences,
	})
	if err != nil {
		return CreateSubmissionOutput{}, err
	}

	output := CreateSubmissionOutput{SubmissionID: submissionID, Status: status}
	if !s.hasAIClient {
		return output, nil
	}

	aiReq := dto.AIRecommendationRequest{SubmissionID: submissionID, UserID: input.UserID, Preferences: aiPrefs}
	if transcriptDoc != nil {
		aiReq.TranscriptURL = transcriptDoc.PublicURL
	}
	if cvDoc != nil {
		aiReq.CVURL = cvDoc.PublicURL
	}

	aiResp, err := s.aiClient.GenerateRecommendations(ctx, aiReq)
	if err != nil {
		_ = s.repo.UpdateSubmissionStatus(ctx, submissionID, input.UserID, model.RecommendationStatusFailed)
		output.Status = model.RecommendationStatusFailed
		return output, nil
	}

	resultSetID := uuid.NewString()
	rows := make([]model.RecommendationResult, 0, len(aiResp.Results))
	for _, item := range aiResp.Results {
		prosJSON, _ := json.Marshal(item.Pros)
		consJSON, _ := json.Marshal(item.Cons)
		rows = append(rows, model.RecommendationResult{
			RecResultID:       uuid.NewString(),
			ResultSetID:       resultSetID,
			RankNo:            item.RankNo,
			UniversityName:    item.UniversityName,
			ProgramName:       item.ProgramName,
			Country:           item.Country,
			FitScore:          item.FitScore,
			FitLevel:          item.FitLevel,
			Overview:          item.Overview,
			WhyThisUniversity: item.WhyThisUniversity,
			WhyThisProgram:    item.WhyThisProgram,
			ReasonSummary:     item.ReasonSummary,
			ProsJSON:          string(prosJSON),
			ConsJSON:          string(consJSON),
			CreatedAt:         now,
		})
	}

	if len(rows) > 0 {
		_, err := s.repo.CreateResultSet(ctx, submissionID, aiResp.GeneratedAt, rows)
		if err != nil {
			_ = s.repo.UpdateSubmissionStatus(ctx, submissionID, input.UserID, model.RecommendationStatusFailed)
			output.Status = model.RecommendationStatusFailed
			return output, nil
		}
		output.ResultSetID = resultSetID
	}

	if err := s.repo.UpdateSubmissionStatus(ctx, submissionID, input.UserID, model.RecommendationStatusCompleted); err != nil {
		return CreateSubmissionOutput{}, err
	}
	output.Status = model.RecommendationStatusCompleted

	return output, nil
}

func (s *RecommendationService) GetSubmissionDetail(ctx context.Context, userID, submissionID string) (repository.SubmissionDetail, error) {
	if userID == "" {
		return repository.SubmissionDetail{}, errs.ErrUnauthorized
	}
	return s.repo.FindSubmissionDetail(ctx, submissionID, userID)
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
