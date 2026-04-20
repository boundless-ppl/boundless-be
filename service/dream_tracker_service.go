package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
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

const reviewRequirementPath = "/dream-tracker/requirements/review"

type CreateDreamTrackerInput struct {
	UserID            string
	ProgramID         string
	AdmissionID       *string
	FundingID         *string
	ScholarshipName   *string
	Title             string
	Status            string
	SourceType        string
	ReqSubmissionID   *string
	SourceRecResultID *string
}

type CreateDreamTrackerOutput struct {
	DreamTracker model.DreamTracker
}

type DreamTrackerDashboardSummary struct {
	TotalTrackers        int
	IncompleteTrackers   int
	CompletedTrackers    int
	NearDeadlineTrackers int
}

type UploadDreamRequirementDocumentInput struct {
	UserID           string
	DreamReqStatusID string
	DocumentType     string
	ReuseIfExists    bool
	File             *multipart.FileHeader
}

type UploadDreamRequirementDocumentOutput struct {
	Requirement model.DreamRequirementStatus
	Document    *model.Document
	Review      model.DreamRequirementReview
}

type DreamTrackerGroupItem struct {
	DreamTrackerID       string
	Title                string
	ProgramName          string
	AdmissionName        string
	UniversityName       string
	Status               model.DreamTrackerStatus
	StatusLabel          string
	CompletionPercentage int
	IsSelected           bool
}

type DreamTrackerUniversityGroup struct {
	UniversityID   string
	UniversityName string
	Items          []DreamTrackerGroupItem
}

type DreamTrackerFundingGroup struct {
	FundingID   string
	FundingName string
	Items       []DreamTrackerGroupItem
}

type GroupedDreamTrackersOutput struct {
	DefaultSelectedDreamTrackerID *string
	Universities                  []DreamTrackerUniversityGroup
	Fundings                      []DreamTrackerFundingGroup
	DefaultDetail                 *repository.DreamTrackerDetail
}

type SubmitDreamRequirementInput struct {
	UserID           string
	DreamReqStatusID string
	DocumentID       string
}

type SubmitDreamRequirementOutput struct {
	Requirement model.DreamRequirementStatus
	AIMessages  []string
	ReviewMeta  *dto.DreamRequirementReviewMeta
}

type DreamTrackerReviewer interface {
	ReviewRequirement(ctx context.Context, req dto.DreamRequirementReviewRequest) (dto.DreamRequirementReviewResponse, error)
}

type HTTPDreamTrackerAIClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPDreamTrackerAIClient(baseURL string) *HTTPDreamTrackerAIClient {
	return &HTTPDreamTrackerAIClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
		},
	}
}

func (c *HTTPDreamTrackerAIClient) ReviewRequirement(ctx context.Context, req dto.DreamRequirementReviewRequest) (dto.DreamRequirementReviewResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return dto.DreamRequirementReviewResponse{}, fmt.Errorf("marshal review requirement payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+reviewRequirementPath, strings.NewReader(string(payload)))
	if err != nil {
		return dto.DreamRequirementReviewResponse{}, fmt.Errorf("build review requirement request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return dto.DreamRequirementReviewResponse{}, fmt.Errorf("call dream requirement AI service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return dto.DreamRequirementReviewResponse{}, fmt.Errorf("dream requirement AI service status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var review dto.DreamRequirementReviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&review); err != nil {
		return dto.DreamRequirementReviewResponse{}, fmt.Errorf("decode review requirement response: %w", err)
	}

	return review, nil
}

type DreamTrackerService struct {
	repo        repository.DreamTrackerRepository
	storage     DocumentStorage
	aiClient    DreamTrackerReviewer
	hasAIClient bool
}

func NewDreamTrackerService(repo repository.DreamTrackerRepository) *DreamTrackerService {
	aiURL := strings.TrimSpace(os.Getenv("AI_SERVICE_URL"))
	service := &DreamTrackerService{
		repo:    repo,
		storage: mustBuildDocumentStorage(),
	}
	if aiURL != "" {
		service.aiClient = NewHTTPDreamTrackerAIClient(aiURL)
		service.hasAIClient = true
	}
	return service
}

func NewDreamTrackerServiceWithDeps(repo repository.DreamTrackerRepository, aiClient DreamTrackerReviewer, storage ...DocumentStorage) *DreamTrackerService {
	var uploader DocumentStorage
	if len(storage) > 0 {
		uploader = storage[0]
	}
	if uploader == nil {
		uploader = mustBuildDocumentStorage()
	}
	return &DreamTrackerService{
		repo:        repo,
		storage:     uploader,
		aiClient:    aiClient,
		hasAIClient: aiClient != nil,
	}
}

func (s *DreamTrackerService) CreateDreamTracker(ctx context.Context, input CreateDreamTrackerInput) (CreateDreamTrackerOutput, error) {
	if input.UserID == "" || strings.TrimSpace(input.SourceType) == "" {
		return CreateDreamTrackerOutput{}, errs.ErrInvalidInput
	}

	trimmedProgramID := strings.TrimSpace(input.ProgramID)
	trimmedTitle := strings.TrimSpace(input.Title)
	scholarshipRequested := input.ScholarshipName != nil || input.FundingID != nil
	seedNeeded := trimmedProgramID == "" || trimmedTitle == "" || input.AdmissionID == nil || (scholarshipRequested && input.FundingID == nil)
	if seedNeeded {
		seed, err := s.repo.ResolveDreamTrackerSeed(ctx, nullableTrimmedString(trimmedProgramID), input.SourceRecResultID, nullableTrimmedStringPtr(input.ScholarshipName))
		if err != nil {
			return CreateDreamTrackerOutput{}, err
		}
		if trimmedProgramID == "" {
			trimmedProgramID = seed.ProgramID
		}
		if trimmedTitle == "" {
			trimmedTitle = seed.Title
		}
		if input.AdmissionID == nil {
			input.AdmissionID = seed.AdmissionID
		}
		if scholarshipRequested && input.FundingID == nil {
			input.FundingID = seed.FundingID
		}
	}
	if trimmedProgramID == "" || trimmedTitle == "" {
		return CreateDreamTrackerOutput{}, errs.ErrInvalidInput
	}

	status := model.DreamTrackerStatusActive
	if strings.TrimSpace(input.Status) != "" {
		status = model.DreamTrackerStatus(strings.TrimSpace(input.Status))
	}

	now := time.Now().UTC()
	tracker := model.DreamTracker{
		DreamTrackerID:    uuid.NewString(),
		UserID:            input.UserID,
		ProgramID:         trimmedProgramID,
		AdmissionID:       input.AdmissionID,
		FundingID:         input.FundingID,
		Title:             trimmedTitle,
		Status:            status,
		CreatedAt:         now,
		UpdatedAt:         now,
		SourceType:        strings.TrimSpace(input.SourceType),
		ReqSubmissionID:   input.ReqSubmissionID,
		SourceRecResultID: input.SourceRecResultID,
	}

	created, err := s.repo.CreateDreamTracker(ctx, tracker)
	if err != nil {
		return CreateDreamTrackerOutput{}, err
	}

	return CreateDreamTrackerOutput{DreamTracker: created}, nil
}

func nullableTrimmedString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func nullableTrimmedStringPtr(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func (s *DreamTrackerService) GetDreamTrackerDetail(ctx context.Context, userID, dreamTrackerID string) (repository.DreamTrackerDetail, error) {
	if userID == "" || strings.TrimSpace(dreamTrackerID) == "" {
		return repository.DreamTrackerDetail{}, errs.ErrInvalidInput
	}
	detail, err := s.repo.FindDreamTrackerDetail(ctx, dreamTrackerID, userID)
	if err != nil {
		return repository.DreamTrackerDetail{}, err
	}
	detail.Summary = buildDreamTrackerSummary(detail.Requirements, detail.Milestones, detail.ProgramInfo.AdmissionDeadline)
	return detail, nil
}

func (s *DreamTrackerService) ListDreamTrackers(ctx context.Context, userID string) ([]repository.DreamTrackerDetail, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errs.ErrInvalidInput
	}

	trackers, err := s.repo.FindDreamTrackersByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]repository.DreamTrackerDetail, 0, len(trackers))
	for _, tracker := range trackers {
		detail, detailErr := s.repo.FindDreamTrackerDetail(ctx, tracker.DreamTrackerID, userID)
		if detailErr != nil {
			return nil, detailErr
		}
		detail.Summary = buildDreamTrackerSummary(detail.Requirements, detail.Milestones, detail.ProgramInfo.AdmissionDeadline)
		items = append(items, detail)
	}

	return items, nil
}

func (s *DreamTrackerService) GetGroupedDreamTrackers(
	ctx context.Context,
	userID string,
	selectedDreamTrackerID *string,
	includeDefaultDetail bool,
) (GroupedDreamTrackersOutput, error) {
	if strings.TrimSpace(userID) == "" {
		return GroupedDreamTrackersOutput{}, errs.ErrInvalidInput
	}

	items, err := s.ListDreamTrackers(ctx, userID)
	if err != nil {
		return GroupedDreamTrackersOutput{}, err
	}

	output := GroupedDreamTrackersOutput{
		Universities: []DreamTrackerUniversityGroup{},
		Fundings:     []DreamTrackerFundingGroup{},
	}
	selectedID := strings.TrimSpace(valueOrEmptyString(selectedDreamTrackerID))
	if selectedID == "" && len(items) > 0 {
		selectedID = items[0].DreamTracker.DreamTrackerID
	}
	if selectedID != "" {
		output.DefaultSelectedDreamTrackerID = &selectedID
	}

	universities := map[string]*DreamTrackerUniversityGroup{}
	fundings := map[string]*DreamTrackerFundingGroup{}
	universityOrder := make([]string, 0)
	fundingOrder := make([]string, 0)

	for _, item := range items {
		groupItem := buildDreamTrackerGroupItem(item, selectedID)

		universityKey := firstNonEmpty(valueOrEmptyString(item.ProgramInfo.UniversityName), item.DreamTracker.Title, item.DreamTracker.DreamTrackerID)
		if _, ok := universities[universityKey]; !ok {
			universityOrder = append(universityOrder, universityKey)
			universities[universityKey] = &DreamTrackerUniversityGroup{
				UniversityName: universityKey,
			}
		}
		universities[universityKey].Items = append(universities[universityKey].Items, groupItem)

		if item.DreamTracker.FundingID != nil && strings.TrimSpace(*item.DreamTracker.FundingID) != "" {
			selectedFundingID := strings.TrimSpace(*item.DreamTracker.FundingID)
			selectedFundingName := selectedFundingID
			for _, funding := range item.Fundings {
				if strings.TrimSpace(funding.FundingID) != selectedFundingID {
					continue
				}
				selectedFundingName = firstNonEmpty(funding.NamaBeasiswa, selectedFundingID)
				break
			}

			if _, ok := fundings[selectedFundingID]; !ok {
				fundingOrder = append(fundingOrder, selectedFundingID)
				fundings[selectedFundingID] = &DreamTrackerFundingGroup{
					FundingID:   selectedFundingID,
					FundingName: selectedFundingName,
				}
			}
			fundings[selectedFundingID].Items = append(fundings[selectedFundingID].Items, groupItem)
		}
	}

	for _, key := range universityOrder {
		output.Universities = append(output.Universities, *universities[key])
	}
	for _, key := range fundingOrder {
		output.Fundings = append(output.Fundings, *fundings[key])
	}

	if includeDefaultDetail && selectedID != "" {
		detail, err := s.GetDreamTrackerDetail(ctx, userID, selectedID)
		if err == nil {
			output.DefaultDetail = &detail
		}
	}

	return output, nil
}

func (s *DreamTrackerService) GetDreamTrackerDashboardSummary(ctx context.Context, userID string) (DreamTrackerDashboardSummary, error) {
	if strings.TrimSpace(userID) == "" {
		return DreamTrackerDashboardSummary{}, errs.ErrInvalidInput
	}

	items, err := s.ListDreamTrackers(ctx, userID)
	if err != nil {
		return DreamTrackerDashboardSummary{}, err
	}

	summary := DreamTrackerDashboardSummary{TotalTrackers: len(items)}
	for _, item := range items {
		if isTrackerCompleted(item) {
			summary.CompletedTrackers++
		} else if item.DreamTracker.Status != model.DreamTrackerStatusArchived {
			summary.IncompleteTrackers++
		}
		if item.Summary.IsDeadlineNear && !isTrackerCompleted(item) {
			summary.NearDeadlineTrackers++
		}
	}

	return summary, nil
}

func (s *DreamTrackerService) GetDocumentDetail(ctx context.Context, userID, documentID string) (model.Document, error) {
	if userID == "" || strings.TrimSpace(documentID) == "" {
		return model.Document{}, errs.ErrInvalidInput
	}
	return s.repo.FindDocumentByIDAndUser(ctx, documentID, userID)
}

func (s *DreamTrackerService) SubmitDreamRequirement(ctx context.Context, input SubmitDreamRequirementInput) (SubmitDreamRequirementOutput, error) {
	if input.UserID == "" || strings.TrimSpace(input.DreamReqStatusID) == "" || strings.TrimSpace(input.DocumentID) == "" {
		return SubmitDreamRequirementOutput{}, errs.ErrInvalidInput
	}

	requirement, err := s.repo.FindDreamRequirementStatusByIDAndUser(ctx, input.DreamReqStatusID, input.UserID)
	if err != nil {
		return SubmitDreamRequirementOutput{}, err
	}

	doc, err := s.repo.FindDocumentByIDAndUser(ctx, input.DocumentID, input.UserID)
	if err != nil {
		return SubmitDreamRequirementOutput{}, err
	}

	requirement.DocumentID = &doc.DocumentID
	requirement.Status = model.DreamRequirementStatusUploaded
	pending := "PENDING"
	requirement.AIStatus = &pending
	requirement.AIMessages = nil

	if err := s.repo.UpdateDreamRequirementStatus(ctx, requirement); err != nil {
		return SubmitDreamRequirementOutput{}, err
	}

	output := SubmitDreamRequirementOutput{
		Requirement: requirement,
		AIMessages:  []string{},
	}

	if !s.hasAIClient {
		return output, nil
	}

	requirementLabel := s.findRequirementLabel(ctx, input.UserID, requirement)

	review, err := s.aiClient.ReviewRequirement(ctx, dto.DreamRequirementReviewRequest{
		DreamReqStatusID:     requirement.DreamReqStatusID,
		DreamTrackerID:       requirement.DreamTrackerID,
		ReqCatalogID:         requirement.ReqCatalogID,
		DocumentID:           doc.DocumentID,
		DocumentURL:          resolveReviewDocumentURL(doc),
		MIMEType:             doc.MIMEType,
		RequiredDocumentType: strings.TrimSpace(string(doc.DocumentType)),
		RequirementLabel:     requirementLabel,
	})
	if err != nil {
		failed := "FAILED"
		message := err.Error()
		requirement.Status = model.DreamRequirementStatusNeedsReview
		requirement.AIStatus = &failed
		requirement.AIMessages = &message
		_ = s.repo.UpdateDreamRequirementStatus(ctx, requirement)
		output.Requirement = requirement
		output.AIMessages = []string{message}
		return output, nil
	}

	if strings.TrimSpace(review.Status) != "" {
		requirement.Status = model.DreamRequirementStatusValue(strings.TrimSpace(review.Status))
	}
	if strings.TrimSpace(review.AIStatus) != "" {
		value := strings.TrimSpace(review.AIStatus)
		requirement.AIStatus = &value
	} else {
		requirement.AIStatus = nil
	}
	if review.Meta != nil && review.Meta.UserMessage != nil && strings.TrimSpace(*review.Meta.UserMessage) != "" {
		value := strings.TrimSpace(*review.Meta.UserMessage)
		requirement.Notes = &value
	} else {
		requirement.Notes = nil
	}
	if len(review.AIMessages) > 0 {
		raw, marshalErr := json.Marshal(review.AIMessages)
		if marshalErr == nil {
			value := string(raw)
			requirement.AIMessages = &value
		}
		output.AIMessages = review.AIMessages
	} else {
		requirement.AIMessages = nil
		output.AIMessages = []string{}
	}
	output.ReviewMeta = review.Meta

	if err := s.repo.UpdateDreamRequirementStatus(ctx, requirement); err != nil {
		return SubmitDreamRequirementOutput{}, err
	}

	output.Requirement = requirement
	return output, nil
}

func (s *DreamTrackerService) UploadDreamRequirementDocument(ctx context.Context, input UploadDreamRequirementDocumentInput) (UploadDreamRequirementDocumentOutput, int, error) {
	if input.UserID == "" || strings.TrimSpace(input.DreamReqStatusID) == "" || strings.TrimSpace(input.DocumentType) == "" {
		return UploadDreamRequirementDocumentOutput{}, http.StatusBadRequest, errs.ErrInvalidInput
	}

	requirement, err := s.repo.FindDreamRequirementStatusByIDAndUser(ctx, input.DreamReqStatusID, input.UserID)
	if err != nil {
		return UploadDreamRequirementDocumentOutput{}, http.StatusBadRequest, err
	}

	normalizedType := canonicalizeDreamRequirementDocumentType(input.DocumentType)
	if normalizedType == "" {
		normalizedType = strings.TrimSpace(input.DocumentType)
	}
	if input.ReuseIfExists {
		if existing, ok := s.currentRequirementDocumentMatches(ctx, input.UserID, requirement, normalizedType); ok {
			msg := "Dokumen ini sudah ada dan tidak perlu diproses ulang."
			review := buildUploadReview("SKIPPED_ALREADY_VERIFIED", "SKIPPED", true, true, &msg, existing.UploadedAt)
			return UploadDreamRequirementDocumentOutput{
				Requirement: requirement,
				Document:    existing,
				Review:      review,
			}, http.StatusOK, nil
		}
	}

	if err := validateUploadHeader(input.File); err != nil {
		return UploadDreamRequirementDocumentOutput{}, http.StatusBadRequest, err
	}

	stored, err := s.storage.Upload(ctx, UploadInput{
		UserID:       input.UserID,
		DocumentType: model.DocumentType(normalizedType),
		Header:       input.File,
	})
	if err != nil {
		return UploadDreamRequirementDocumentOutput{}, http.StatusInternalServerError, err
	}

	doc := model.Document{
		DocumentID:       uuid.NewString(),
		UserID:           input.UserID,
		OriginalFilename: input.File.Filename,
		StoragePath:      stored.StoragePath,
		PublicURL:        stored.PublicURL,
		MIMEType:         stored.MIMEType,
		SizeBytes:        stored.SizeBytes,
		DocumentType:     model.DocumentType(normalizedType),
		UploadedAt:       time.Now().UTC(),
	}
	createdDoc, err := s.repo.CreateDocument(ctx, doc)
	if err != nil {
		return UploadDreamRequirementDocumentOutput{}, http.StatusInternalServerError, err
	}

	submitOutput, err := s.SubmitDreamRequirement(ctx, SubmitDreamRequirementInput{
		UserID:           input.UserID,
		DreamReqStatusID: input.DreamReqStatusID,
		DocumentID:       createdDoc.DocumentID,
	})
	if err != nil {
		return UploadDreamRequirementDocumentOutput{}, http.StatusBadRequest, err
	}

	review := buildUploadReview(
		"NEW_UPLOAD",
		uploadReviewStatus(submitOutput.Requirement),
		false,
		submitOutput.Requirement.Status == model.DreamRequirementStatusVerified || submitOutput.Requirement.Status == model.DreamRequirementStatusVerifiedWithWarning,
		selectRequirementMessage(submitOutput.Requirement.Notes, submitOutput.AIMessages),
		createdDoc.UploadedAt,
	)
	statusCode := http.StatusCreated
	if review.Status == "PROCESSING" || review.Status == "PENDING" {
		statusCode = http.StatusAccepted
	}

	return UploadDreamRequirementDocumentOutput{
		Requirement: submitOutput.Requirement,
		Document:    &createdDoc,
		Review:      review,
	}, statusCode, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func resolveReviewDocumentURL(doc model.Document) string {
	publicURL := strings.TrimSpace(firstNonEmpty(doc.DokumenURL, doc.PublicURL))
	localPath := resolveLocalDocumentPath(doc.StoragePath)

	if useLocalDocumentPathForAI() && localPath != "" {
		return localPath
	}
	if publicURL != "" {
		return publicURL
	}
	return localPath
}

func useLocalDocumentPathForAI() bool {
	raw := strings.TrimSpace(os.Getenv("AI_SERVICE_URL"))
	if raw == "" {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return false
	}
	return strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1"
}

func resolveLocalDocumentPath(storagePath string) string {
	trimmed := strings.TrimSpace(storagePath)
	if trimmed == "" {
		return ""
	}
	if filepath.IsAbs(trimmed) {
		return trimmed
	}
	baseDir := strings.TrimSpace(os.Getenv("DOCUMENT_STORAGE_DIR"))
	if baseDir == "" {
		baseDir = "uploads"
	}
	joined := filepath.Join(baseDir, trimmed)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return joined
	}
	return abs
}

func (s *DreamTrackerService) findRequirementLabel(
	ctx context.Context,
	userID string,
	requirement model.DreamRequirementStatus,
) string {
	detail, err := s.repo.FindDreamTrackerDetail(ctx, requirement.DreamTrackerID, userID)
	if err != nil {
		return ""
	}
	for _, item := range detail.Requirements {
		if item.DreamReqStatusID == requirement.DreamReqStatusID {
			return strings.TrimSpace(item.RequirementLabel)
		}
	}
	return ""
}

func canonicalizeDreamRequirementDocumentType(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	upper := strings.ToUpper(trimmed)
	switch upper {
	case "TRANSCRIPT", "PASSPORT", "KTP", "KK", "DIPLOMA", "DUOLINGO_CERT", "RECOMMENDATION_LETTER", "OFFER_LETTER", "SCHOLARSHIP_LETTER", "BANK_STATEMENT", "SPONSORSHIP_LETTER", "VISA_LETTER":
		return upper
	}

	normalized := strings.ToLower(trimmed)
	switch {
	case strings.Contains(normalized, "transcript"), strings.Contains(normalized, "transkrip"):
		return "TRANSCRIPT"
	case strings.Contains(normalized, "passport"), strings.Contains(normalized, "paspor"):
		return "PASSPORT"
	case strings.Contains(normalized, "ktp"), strings.Contains(normalized, "identity"), strings.Contains(normalized, "identitas"):
		return "KTP"
	case strings.Contains(normalized, "kartu keluarga"), strings.Contains(normalized, "family card"), normalized == "kk":
		return "KK"
	case strings.Contains(normalized, "diploma"), strings.Contains(normalized, "ijazah"):
		return "DIPLOMA"
	case strings.Contains(normalized, "duolingo"):
		return "DUOLINGO_CERT"
	case strings.Contains(normalized, "surat rekomendasi"), strings.Contains(normalized, "recommendation letter"), strings.Contains(normalized, "letter of recommendation"), strings.Contains(normalized, "reference letter"), strings.Contains(normalized, "rekomendasi"), strings.Contains(normalized, "lor"):
		return "RECOMMENDATION_LETTER"
	case strings.Contains(normalized, "offer letter"), strings.Contains(normalized, "acceptance letter"), strings.Contains(normalized, "letter of acceptance"):
		return "OFFER_LETTER"
	case strings.Contains(normalized, "scholarship"), strings.Contains(normalized, "award letter"):
		return "SCHOLARSHIP_LETTER"
	case strings.Contains(normalized, "bank statement"), strings.Contains(normalized, "rekening koran"), strings.Contains(normalized, "financial statement"),
		strings.Contains(normalized, "financial support"), strings.Contains(normalized, "financial proof"), normalized == "financial_proof":
		return "BANK_STATEMENT"
	case strings.Contains(normalized, "sponsorship"):
		return "SPONSORSHIP_LETTER"
	case strings.Contains(normalized, "visa"):
		return "VISA_LETTER"
	default:
		return trimmed
	}
}

func buildDreamTrackerGroupItem(detail repository.DreamTrackerDetail, selectedID string) DreamTrackerGroupItem {
	statusLabel := "Sedang Diproses"
	switch {
	case detail.DreamTracker.Status == model.DreamTrackerStatusCompleted:
		statusLabel = "Selesai"
	case detail.Summary.IsOverdue:
		statusLabel = "Deadline Terlewat"
	case detail.Summary.IsDeadlineNear:
		statusLabel = "Deadline Mendekat"
	}

	return DreamTrackerGroupItem{
		DreamTrackerID:       detail.DreamTracker.DreamTrackerID,
		Title:                detail.DreamTracker.Title,
		ProgramName:          valueOrEmptyString(detail.ProgramInfo.ProgramName),
		AdmissionName:        valueOrEmptyString(detail.ProgramInfo.AdmissionName),
		UniversityName:       valueOrEmptyString(detail.ProgramInfo.UniversityName),
		Status:               detail.DreamTracker.Status,
		StatusLabel:          statusLabel,
		CompletionPercentage: detail.Summary.CompletionPercentage,
		IsSelected:           detail.DreamTracker.DreamTrackerID == selectedID,
	}
}

func (s *DreamTrackerService) currentRequirementDocumentMatches(
	ctx context.Context,
	userID string,
	requirement model.DreamRequirementStatus,
	documentType string,
) (*model.Document, bool) {
	if requirement.DocumentID == nil || (requirement.Status != model.DreamRequirementStatusVerified && requirement.Status != model.DreamRequirementStatusVerifiedWithWarning) {
		return nil, false
	}

	doc, err := s.repo.FindDocumentByIDAndUser(ctx, *requirement.DocumentID, userID)
	if err != nil {
		return nil, false
	}
	if !strings.EqualFold(string(doc.DocumentType), documentType) {
		return nil, false
	}
	return &doc, true
}

func buildUploadReview(source, status string, isReused, isAlreadyVerified bool, message *string, processedAt time.Time) model.DreamRequirementReview {
	review := model.DreamRequirementReview{
		Source:            source,
		Status:            status,
		IsReused:          isReused,
		IsAlreadyVerified: isAlreadyVerified,
		AIMessage:         message,
	}
	if !processedAt.IsZero() {
		value := processedAt.UTC()
		review.LastProcessedAt = &value
	}
	return review
}

func uploadReviewStatus(requirement model.DreamRequirementStatus) string {
	switch requirement.Status {
	case model.DreamRequirementStatusReviewing:
		return "PROCESSING"
	case model.DreamRequirementStatusReused:
		return "SKIPPED"
	case model.DreamRequirementStatusUploaded:
		if strings.EqualFold(valueOrEmptyString(requirement.AIStatus), "PENDING") {
			return "PENDING"
		}
		return "COMPLETED"
	case model.DreamRequirementStatusVerified:
		return "COMPLETED"
	case model.DreamRequirementStatusVerifiedWithWarning:
		return "COMPLETED"
	case model.DreamRequirementStatusRejected:
		return "FAILED"
	default:
		return "NOT_STARTED"
	}
}

func stringPtr(value string) *string {
	return &value
}

func mustMarshalMessages(messages []string) string {
	payload, _ := json.Marshal(messages)
	return string(payload)
}

func valueOrEmptyString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func selectRequirementMessage(notes *string, aiMessages []string) *string {
	if notes != nil && strings.TrimSpace(*notes) != "" {
		return notes
	}
	if len(aiMessages) > 0 && strings.TrimSpace(aiMessages[0]) != "" {
		return &aiMessages[0]
	}
	return nil
}

func buildDreamTrackerSummary(
	requirements []model.DreamRequirementDetail,
	milestones []model.DreamKeyMilestone,
	admissionDeadline *time.Time,
) model.DreamTrackerSummary {
	summary := model.DreamTrackerSummary{
		TotalRequirements: len(requirements),
	}
	summary.CompletedRequirements = countCompletedRequirements(requirements)
	summary.CompletionPercentage = calculateCompletionPercentage(summary.CompletedRequirements, summary.TotalRequirements)

	now := time.Now().UTC()
	nextDeadline := nextUpcomingDeadline(milestones, now)
	if nextDeadline == nil {
		nextDeadline = admissionDeadline
	}
	summary.NextDeadlineAt = nextDeadline
	if nextDeadline != nil {
		summary.IsOverdue = nextDeadline.Before(now)
		summary.IsDeadlineNear = !summary.IsOverdue && nextDeadline.Sub(now) <= 7*24*time.Hour
	}
	if !summary.IsOverdue && admissionDeadline != nil && admissionDeadline.Before(now) {
		summary.IsOverdue = true
	}
	return summary
}

func countCompletedRequirements(requirements []model.DreamRequirementDetail) int {
	completed := 0
	for _, requirement := range requirements {
		if isCompletedRequirement(requirement.Status) {
			completed++
		}
	}
	return completed
}

func isCompletedRequirement(status model.DreamRequirementStatusValue) bool {
	return status == model.DreamRequirementStatusUploaded || status == model.DreamRequirementStatusVerified || status == model.DreamRequirementStatusVerifiedWithWarning
}

func isTrackerCompleted(detail repository.DreamTrackerDetail) bool {
	if detail.DreamTracker.Status == model.DreamTrackerStatusCompleted {
		return true
	}
	return detail.Summary.TotalRequirements > 0 && detail.Summary.CompletedRequirements == detail.Summary.TotalRequirements
}

func calculateCompletionPercentage(completed, total int) int {
	if total == 0 {
		return 0
	}
	return (completed * 100) / total
}

func nextUpcomingDeadline(milestones []model.DreamKeyMilestone, now time.Time) *time.Time {
	var nextDeadline *time.Time
	for _, milestone := range milestones {
		if !isUpcomingMilestone(milestone, now) {
			continue
		}
		if nextDeadline == nil || milestone.DeadlineDate.Before(*nextDeadline) {
			nextDeadline = milestone.DeadlineDate
		}
	}
	return nextDeadline
}

func isUpcomingMilestone(milestone model.DreamKeyMilestone, now time.Time) bool {
	return milestone.DeadlineDate != nil && !milestone.DeadlineDate.Before(now)
}

func isPDFDocument(doc model.Document) bool {
	if strings.EqualFold(strings.TrimSpace(doc.MIMEType), "application/pdf") {
		return true
	}
	return strings.EqualFold(filepath.Ext(strings.TrimSpace(doc.OriginalFilename)), ".pdf")
}
