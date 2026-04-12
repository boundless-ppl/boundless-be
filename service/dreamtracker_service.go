package service

import (
	"context"
	"mime/multipart"
	"strings"
	"time"

	"boundless-be/dto"
	"boundless-be/errs"
	"boundless-be/model"
	"boundless-be/repository"

	"github.com/google/uuid"
)

type DreamTrackerService struct {
	repo    repository.DreamTrackerRepository
	storage DocumentStorage
}

func NewDreamTrackerService(repo repository.DreamTrackerRepository) *DreamTrackerService {
	return &DreamTrackerService{
		repo:    repo,
		storage: mustBuildDocumentStorage(),
	}
}

func (s *DreamTrackerService) GetSummary(ctx context.Context, userID string) (dto.DreamTrackerDashboardSummaryResponse, error) {
	rows, err := s.repo.ListTrackerSummaries(ctx, userID)
	if err != nil {
		return dto.DreamTrackerDashboardSummaryResponse{}, err
	}

	now := time.Now().UTC()
	out := dto.DreamTrackerDashboardSummaryResponse{
		TotalApplications: len(rows),
	}
	for _, row := range rows {
		total := row.TotalRequirements
		completed := row.CompletedRequirements
		isCompleted := total > 0 && completed >= total
		if strings.EqualFold(row.Status, "COMPLETED") || isCompleted {
			out.CompletedCount++
		} else {
			out.IncompleteCount++
		}
		if row.AdmissionDeadline != nil {
			days := row.AdmissionDeadline.Sub(now)
			if days >= 0 && days <= 14*24*time.Hour {
				out.DeadlineNearCount++
			}
		}
	}
	return out, nil
}

func (s *DreamTrackerService) GetGrouped(ctx context.Context, userID string, includeDefaultDetail bool, selectedDreamTrackerID string) (dto.DreamTrackerGroupedResponse, error) {
	rows, err := s.repo.ListTrackerSummaries(ctx, userID)
	if err != nil {
		return dto.DreamTrackerGroupedResponse{}, err
	}

	resp := dto.DreamTrackerGroupedResponse{
		Universities: make([]dto.DreamTrackerGroupedUniversityResponse, 0),
		Fundings:     make([]dto.DreamTrackerGroupedFundingResponse, 0),
	}
	if len(rows) == 0 {
		return resp, nil
	}

	defaultID := selectedDreamTrackerID
	if defaultID == "" {
		defaultID = rows[0].DreamTrackerID
	}
	resp.DefaultSelectedDreamTrackerID = defaultID

	univGroups := make(map[string]*dto.DreamTrackerGroupedUniversityResponse)
	univOrder := make([]string, 0)
	for _, row := range rows {
		key := row.UniversityID
		if key == "" {
			key = row.UniversityName
		}
		group := univGroups[key]
		if group == nil {
			group = &dto.DreamTrackerGroupedUniversityResponse{
				UniversityID:   row.UniversityID,
				UniversityName: row.UniversityName,
				Items:          make([]dto.DreamTrackerGroupedUniversityItemResponse, 0),
			}
			univGroups[key] = group
			univOrder = append(univOrder, key)
		}
		group.Items = append(group.Items, dto.DreamTrackerGroupedUniversityItemResponse{
			DreamTrackerID:       row.DreamTrackerID,
			Title:                row.Title,
			ProgramName:          row.ProgramName,
			AdmissionName:        row.AdmissionName,
			Status:               normalizeDreamTrackerStatus(row.Status),
			StatusLabel:          dreamTrackerStatusLabel(row.Status),
			CompletionPercentage: completionPercentage(row.CompletedRequirements, row.TotalRequirements),
			IsSelected:           row.DreamTrackerID == defaultID,
		})
	}
	for _, key := range univOrder {
		resp.Universities = append(resp.Universities, *univGroups[key])
	}

	fundingLinks, err := s.repo.ListFundingLinks(ctx, userID)
	if err != nil {
		return dto.DreamTrackerGroupedResponse{}, err
	}
	fundingGroups := make(map[string]*dto.DreamTrackerGroupedFundingResponse)
	fundingOrder := make([]string, 0)
	for _, row := range fundingLinks {
		group := fundingGroups[row.FundingID]
		if group == nil {
			group = &dto.DreamTrackerGroupedFundingResponse{
				FundingID:   row.FundingID,
				FundingName: row.FundingName,
				Items:       make([]dto.DreamTrackerGroupedFundingItemResponse, 0),
			}
			fundingGroups[row.FundingID] = group
			fundingOrder = append(fundingOrder, row.FundingID)
		}
		group.Items = append(group.Items, dto.DreamTrackerGroupedFundingItemResponse{
			DreamTrackerID:       row.DreamTrackerID,
			Title:                row.Title,
			ProgramName:          row.ProgramName,
			UniversityName:       row.UniversityName,
			Status:               normalizeDreamTrackerStatus(row.Status),
			StatusLabel:          dreamTrackerStatusLabel(row.Status),
			CompletionPercentage: row.CompletionPercentage,
			IsSelected:           row.DreamTrackerID == defaultID,
		})
	}
	for _, key := range fundingOrder {
		resp.Fundings = append(resp.Fundings, *fundingGroups[key])
	}

	if includeDefaultDetail && defaultID != "" {
		detail, err := s.GetByID(ctx, userID, defaultID)
		if err == nil {
			resp.DefaultDetail = &detail
		}
	}

	return resp, nil
}

func (s *DreamTrackerService) GetByID(ctx context.Context, userID, dreamTrackerID string) (dto.DreamTrackerItemResponse, error) {
	base, err := s.repo.GetTrackerSummaryByID(ctx, userID, dreamTrackerID)
	if err != nil {
		return dto.DreamTrackerItemResponse{}, err
	}
	reqs, err := s.repo.ListRequirements(ctx, userID, dreamTrackerID)
	if err != nil {
		return dto.DreamTrackerItemResponse{}, err
	}
	milestones, err := s.repo.ListMilestones(ctx, userID, dreamTrackerID)
	if err != nil {
		return dto.DreamTrackerItemResponse{}, err
	}
	fundings, err := s.repo.ListFundings(ctx, userID, dreamTrackerID)
	if err != nil {
		return dto.DreamTrackerItemResponse{}, err
	}

	requirementItems := make([]dto.DreamRequirementResponse, 0, len(reqs))
	for _, req := range reqs {
		requirementItems = append(requirementItems, toDreamRequirementResponse(req))
	}
	milestoneItems := make([]dto.DreamMilestoneResponse, 0, len(milestones))
	for _, milestone := range milestones {
		milestoneItems = append(milestoneItems, dto.DreamMilestoneResponse{
			DreamMilestoneID: milestone.DreamMilestoneID,
			Title:            milestone.Title,
			Status:           normalizeMilestoneStatus(milestone.Status),
			DeadlineDate:     milestone.DeadlineDate.UTC().Format(time.RFC3339),
		})
	}
	fundingItems := make([]dto.DreamFundingResponse, 0, len(fundings))
	for _, funding := range fundings {
		status := "AVAILABLE"
		if base.FundingID != nil && *base.FundingID == funding.FundingID {
			status = "SELECTED"
		}
		fundingItems = append(fundingItems, dto.DreamFundingResponse{
			FundingID:    funding.FundingID,
			NamaBeasiswa: funding.NamaBeasiswa,
			Provider:     funding.Provider,
			Status:       status,
		})
	}

	summary := buildDreamSummary(base.CompletedRequirements, base.TotalRequirements, base.AdmissionDeadline)
	var deadlineAt *string
	if base.AdmissionDeadline != nil {
		formatted := base.AdmissionDeadline.UTC().Format(time.RFC3339)
		deadlineAt = &formatted
	}

	admissionDeadline := ""
	if base.AdmissionDeadline != nil {
		admissionDeadline = base.AdmissionDeadline.UTC().Format(time.RFC3339)
	}

	return dto.DreamTrackerItemResponse{
		DreamTrackerID: dreamTrackerID,
		Title:          base.Title,
		Subtitle:       strings.TrimSpace(base.AdmissionName),
		Status:         normalizeDreamTrackerStatus(base.Status),
		StatusLabel:    dreamTrackerStatusLabel(base.Status),
		StatusVariant:  dreamTrackerStatusVariant(base.Status),
		CreatedAt:      base.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      base.UpdatedAt.UTC().Format(time.RFC3339),
		DeadlineAt:     deadlineAt,
		Summary:        summary,
		Program: dto.DreamTrackerProgramResponse{
			ProgramID:         base.ProgramID,
			ProgramName:       base.ProgramName,
			UniversityName:    base.UniversityName,
			AdmissionName:     base.AdmissionName,
			Intake:            base.AdmissionName,
			AdmissionURL:      base.AdmissionURL,
			AdmissionDeadline: admissionDeadline,
		},
		Requirements: requirementItems,
		Milestones:   milestoneItems,
		Fundings:     fundingItems,
	}, nil
}

func (s *DreamTrackerService) Create(ctx context.Context, userID string, req dto.CreateDreamTrackerRequest) (dto.CreateDreamTrackerResponse, error) {
	title := strings.TrimSpace(valueOrEmpty(req.Title))
	if title == "" {
		title = req.ProgramID
	}
	id, status, err := s.repo.CreateDreamTracker(ctx, repository.CreateDreamTrackerParams{
		UserID:            userID,
		ProgramID:         req.ProgramID,
		AdmissionID:       trimDreamPtr(req.AdmissionID),
		FundingID:         trimDreamPtr(req.FundingID),
		Title:             title,
		Status:            strings.TrimSpace(valueOrEmpty(req.Status)),
		SourceType:        req.SourceType,
		ReqSubmissionID:   trimDreamPtr(req.ReqSubmissionID),
		SourceRecResultID: trimDreamPtr(req.SourceRecResultID),
	})
	if err != nil {
		return dto.CreateDreamTrackerResponse{}, err
	}
	if err := s.repo.InitializeFundingRequirements(ctx, id, trimDreamPtr(req.FundingID)); err != nil {
		return dto.CreateDreamTrackerResponse{}, err
	}
	return dto.CreateDreamTrackerResponse{
		DreamTrackerID: id,
		Status:         normalizeDreamTrackerStatus(status),
	}, nil
}

func (s *DreamTrackerService) UploadRequirementDocument(ctx context.Context, userID, requirementStatusID, documentType string, file *multipart.FileHeader) (dto.SubmitRequirementResponse, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(requirementStatusID) == "" {
		return dto.SubmitRequirementResponse{}, errs.ErrInvalidInput
	}
	if err := validateUploadHeader(file); err != nil {
		return dto.SubmitRequirementResponse{}, err
	}
	target, err := s.repo.FindRequirementTarget(ctx, userID, requirementStatusID)
	if err != nil {
		return dto.SubmitRequirementResponse{}, err
	}

	stored, err := s.storage.Upload(ctx, UploadInput{
		UserID:       userID,
		DocumentType: model.DocumentType(strings.ToLower(strings.TrimSpace(documentType))),
		Header:       file,
	})
	if err != nil {
		return dto.SubmitRequirementResponse{}, err
	}

	doc := model.Document{
		DocumentID:       uuid.NewString(),
		UserID:           userID,
		OriginalFilename: file.Filename,
		StoragePath:      stored.StoragePath,
		PublicURL:        stored.PublicURL,
		MIMEType:         stored.MIMEType,
		SizeBytes:        stored.SizeBytes,
		DocumentType:     model.DocumentType(strings.ToLower(strings.TrimSpace(documentType))),
		UploadedAt:       time.Now().UTC(),
	}
	created, err := s.repo.CreateDocument(ctx, doc)
	if err != nil {
		return dto.SubmitRequirementResponse{}, err
	}
	if err := s.repo.AttachRequirementDocument(ctx, target, created.DocumentID); err != nil {
		return dto.SubmitRequirementResponse{}, err
	}
	row, err := s.repo.GetRequirementRowByID(ctx, userID, requirementStatusID)
	if err != nil {
		return dto.SubmitRequirementResponse{}, err
	}
	mapped := toDreamRequirementResponse(row)
	return dto.SubmitRequirementResponse{
		DreamReqStatusID: mapped.DreamReqStatusID,
		Status:           mapped.Status,
		StatusLabel:      mapped.StatusLabel,
		StatusVariant:    mapped.StatusVariant,
		Document:         mapped.Document,
		Review:           mapped.Review,
	}, nil
}

func buildDreamSummary(completed, total int, deadline *time.Time) dto.DreamTrackerSummaryDataResponse {
	now := time.Now().UTC()
	pct := completionPercentage(completed, total)
	var nextDeadline *string
	isNear := false
	isOverdue := false
	if deadline != nil {
		formatted := deadline.UTC().Format(time.RFC3339)
		nextDeadline = &formatted
		diff := deadline.Sub(now)
		isNear = diff >= 0 && diff <= 14*24*time.Hour
		isOverdue = diff < 0
	}
	return dto.DreamTrackerSummaryDataResponse{
		CompletionPercentage:  pct,
		CompletedRequirements: completed,
		TotalRequirements:     total,
		NextDeadlineAt:        nextDeadline,
		IsDeadlineNear:        isNear,
		IsOverdue:             isOverdue,
	}
}

func completionPercentage(completed, total int) int {
	if total <= 0 {
		return 0
	}
	return int(float64(completed) / float64(total) * 100)
}

func toDreamRequirementResponse(row repository.DreamRequirementRow) dto.DreamRequirementResponse {
	status, label, variant, canUpload, needsReupload := mapRequirementStatus(row.Status)
	var doc *dto.DreamRequirementDocumentResponse
	if row.Document != nil {
		uploadedAt := row.Document.UploadedAt.UTC().Format(time.RFC3339)
		doc = &dto.DreamRequirementDocumentResponse{
			DocumentID:       row.Document.DocumentID,
			DocumentType:     string(row.Document.DocumentType),
			OriginalFilename: row.Document.OriginalFilename,
			PublicURL:        row.Document.PublicURL,
			MIMEType:         row.Document.MIMEType,
			UploadedAt:       uploadedAt,
		}
	}
	reviewStatus := "NOT_STARTED"
	if row.AIStatus != nil && strings.TrimSpace(*row.AIStatus) != "" {
		reviewStatus = normalizeReviewStatus(*row.AIStatus)
	} else if row.Document != nil {
		reviewStatus = "PENDING"
	}
	return dto.DreamRequirementResponse{
		DreamReqStatusID: row.DreamReqStatusID,
		ReqCatalogID:     row.ReqCatalogID,
		RequirementKey:   row.RequirementKey,
		RequirementLabel: row.RequirementLabel,
		Category:         row.Category,
		Status:           status,
		StatusLabel:      label,
		StatusVariant:    variant,
		CanUpload:        canUpload,
		NeedsReupload:    needsReupload,
		Document:         doc,
		Review: dto.DreamRequirementReviewResponse{
			Source:            reviewSource(status),
			Status:            reviewStatus,
			IsReused:          status == "REUSED",
			IsAlreadyVerified: status == "VERIFIED" || status == "REUSED",
			AIMessage:         row.AIMessages,
			LastProcessedAt:   nil,
		},
	}
}

func mapRequirementStatus(raw string) (status, label, variant string, canUpload, needsReupload bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "VERIFIED":
		return "VERIFIED", "Sudah tersedia", "SUCCESS", false, false
	case "REUSED":
		return "REUSED", "Sudah tersedia", "SUCCESS", false, false
	case "UPLOADED":
		return "UPLOADED", "Sedang diperiksa", "IN_PROGRESS", false, false
	case "REVIEWING":
		return "REVIEWING", "Sedang diperiksa", "IN_PROGRESS", false, false
	case "REJECTED":
		return "REJECTED", "Ditolak", "ERROR", true, true
	case "NEEDS_REVIEW", "VERIFIED_WITH_WARNING":
		return "VERIFIED_WITH_WARNING", "Perlu verifikasi ulang", "WARNING", true, true
	default:
		return "NOT_UPLOADED", "Belum diunggah", "DEFAULT", true, false
	}
}

func normalizeDreamTrackerStatus(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "COMPLETED":
		return "COMPLETED"
	case "ARCHIVED":
		return "ARCHIVED"
	default:
		return "ACTIVE"
	}
}

func dreamTrackerStatusLabel(raw string) string {
	switch normalizeDreamTrackerStatus(raw) {
	case "COMPLETED":
		return "Selesai"
	case "ARCHIVED":
		return "Diarsipkan"
	default:
		return "Sedang Diproses"
	}
}

func dreamTrackerStatusVariant(raw string) string {
	switch normalizeDreamTrackerStatus(raw) {
	case "COMPLETED":
		return "SUCCESS"
	case "ARCHIVED":
		return "DEFAULT"
	default:
		return "IN_PROGRESS"
	}
}

func normalizeMilestoneStatus(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DONE", "COMPLETED":
		return "DONE"
	case "MISSED", "OVERDUE":
		return "MISSED"
	default:
		return "NOT_STARTED"
	}
}

func normalizeReviewStatus(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "COMPLETED":
		return "COMPLETED"
	case "FAILED":
		return "FAILED"
	case "PROCESSING":
		return "PROCESSING"
	case "SKIPPED":
		return "SKIPPED"
	default:
		return "PENDING"
	}
}

func reviewSource(status string) string {
	if status == "REUSED" {
		return "REUSED_EXISTING"
	}
	return "NEW_UPLOAD"
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func trimDreamPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
