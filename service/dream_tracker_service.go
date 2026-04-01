package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

const dreamRequirementReviewPath = "/dream-tracker/requirements/review"

type CreateDreamTrackerInput struct {
	UserID            string
	ProgramID         string
	AdmissionID       *string
	FundingID         *string
	Title             string
	Status            string
	SourceType        string
	ReqSubmissionID   *string
	SourceRecResultID *string
}

type CreateDreamTrackerOutput struct {
	DreamTracker model.DreamTracker
}

type SubmitDreamRequirementInput struct {
	UserID           string
	DreamReqStatusID string
	DocumentID       string
}

type SubmitDreamRequirementOutput struct {
	Requirement model.DreamRequirementStatus
	AIMessages  []string
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
			Timeout: 60 * time.Second,
		},
	}
}

func (c *HTTPDreamTrackerAIClient) ReviewRequirement(ctx context.Context, req dto.DreamRequirementReviewRequest) (dto.DreamRequirementReviewResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return dto.DreamRequirementReviewResponse{}, fmt.Errorf("marshal dream requirement review request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+dreamRequirementReviewPath, bytes.NewReader(body))
	if err != nil {
		return dto.DreamRequirementReviewResponse{}, fmt.Errorf("build dream requirement review request: %w", err)
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
		return dto.DreamRequirementReviewResponse{}, fmt.Errorf("decode dream requirement review response: %w", err)
	}

	return review, nil
}

type DreamTrackerService struct {
	repo        repository.DreamTrackerRepository
	aiClient    DreamTrackerReviewer
	hasAIClient bool
}

func NewDreamTrackerService(repo repository.DreamTrackerRepository) *DreamTrackerService {
	aiURL := strings.TrimSpace(os.Getenv("AI_SERVICE_URL"))
	service := &DreamTrackerService{repo: repo}
	if aiURL != "" {
		service.aiClient = NewHTTPDreamTrackerAIClient(aiURL)
		service.hasAIClient = true
	}
	return service
}

func NewDreamTrackerServiceWithDeps(repo repository.DreamTrackerRepository, aiClient DreamTrackerReviewer) *DreamTrackerService {
	return &DreamTrackerService{
		repo:        repo,
		aiClient:    aiClient,
		hasAIClient: aiClient != nil,
	}
}

func (s *DreamTrackerService) CreateDreamTracker(ctx context.Context, input CreateDreamTrackerInput) (CreateDreamTrackerOutput, error) {
	if input.UserID == "" || strings.TrimSpace(input.ProgramID) == "" || strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.SourceType) == "" {
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
		ProgramID:         strings.TrimSpace(input.ProgramID),
		AdmissionID:       input.AdmissionID,
		FundingID:         input.FundingID,
		Title:             strings.TrimSpace(input.Title),
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
	if !isPDFDocument(doc) {
		return SubmitDreamRequirementOutput{}, errs.ErrInvalidInput
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

	review, err := s.aiClient.ReviewRequirement(ctx, dto.DreamRequirementReviewRequest{
		DreamReqStatusID: requirement.DreamReqStatusID,
		DreamTrackerID:   requirement.DreamTrackerID,
		ReqCatalogID:     requirement.ReqCatalogID,
		DocumentID:       doc.DocumentID,
		DocumentURL:      firstNonEmpty(doc.DokumenURL, doc.PublicURL),
		MIMEType:         doc.MIMEType,
	})
	if err != nil {
		failed := "FAILED"
		message := err.Error()
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
	}
	if len(review.AIMessages) > 0 {
		raw, marshalErr := json.Marshal(review.AIMessages)
		if marshalErr == nil {
			value := string(raw)
			requirement.AIMessages = &value
		}
		output.AIMessages = review.AIMessages
	}

	if err := s.repo.UpdateDreamRequirementStatus(ctx, requirement); err != nil {
		return SubmitDreamRequirementOutput{}, err
	}

	output.Requirement = requirement
	return output, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
	return status == model.DreamRequirementStatusUploaded || status == model.DreamRequirementStatusVerified
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
