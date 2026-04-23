package presenter

import (
	"encoding/json"
	"strings"
	"time"

	"boundless-be/dto"
	"boundless-be/model"
	"boundless-be/repository"
	"boundless-be/service"
)

type DreamTrackerPresenter struct {
	trackerStatusPolicy     TrackerStatusPolicy
	requirementStatusPolicy RequirementStatusPolicy
}

func NewDreamTrackerPresenter() DreamTrackerPresenter {
	return DreamTrackerPresenter{
		trackerStatusPolicy:     DefaultTrackerStatusPolicy{},
		requirementStatusPolicy: DefaultRequirementStatusPolicy{},
	}
}

func (p DreamTrackerPresenter) PresentTracker(detail repository.DreamTrackerDetail) dto.DreamTrackerResponse {
	deadlineAt := detail.Summary.NextDeadlineAt
	if deadlineAt == nil {
		deadlineAt = detail.ProgramInfo.AdmissionDeadline
	}

	var deadline *string
	if deadlineAt != nil {
		value := deadlineAt.UTC().Format(time.RFC3339)
		deadline = &value
	}

	trackerPresentation := p.trackerStatusPolicy.Present(detail)

	return dto.DreamTrackerResponse{
		DreamTrackerID:    detail.DreamTracker.DreamTrackerID,
		UserID:            detail.DreamTracker.UserID,
		ProgramID:         detail.DreamTracker.ProgramID,
		AdmissionID:       detail.DreamTracker.AdmissionID,
		FundingID:         detail.DreamTracker.FundingID,
		Title:             detail.DreamTracker.Title,
		Subtitle:          dreamTrackerSubtitle(detail),
		Status:            string(detail.DreamTracker.Status),
		StatusLabel:       trackerPresentation.Label,
		StatusVariant:     trackerPresentation.Variant,
		CreatedAt:         detail.DreamTracker.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         detail.DreamTracker.UpdatedAt.UTC().Format(time.RFC3339),
		SourceType:        detail.DreamTracker.SourceType,
		ReqSubmissionID:   detail.DreamTracker.ReqSubmissionID,
		SourceRecResultID: detail.DreamTracker.SourceRecResultID,
		DeadlineAt:        deadline,
		Progress: dto.DreamTrackerProgressResponse{
			Percentage:         detail.Summary.CompletionPercentage,
			CompletedDocuments: detail.Summary.CompletedRequirements,
			TotalDocuments:     detail.Summary.TotalRequirements,
		},
		Summary:      toDreamTrackerSummaryResponse(detail.Summary),
		Program:      toDreamTrackerProgramInfoResponse(detail.ProgramInfo),
		Requirements: p.presentRequirements(detail.Requirements),
		Milestones:   toDreamTrackerMilestoneResponses(detail.Milestones),
		Fundings:     toDreamTrackerFundingResponses(detail.Fundings),
	}
}

func (p DreamTrackerPresenter) PresentTrackers(items []repository.DreamTrackerDetail) dto.DreamTrackerListResponse {
	responseItems := make([]dto.DreamTrackerResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, p.PresentTracker(item))
	}
	return dto.DreamTrackerListResponse{Items: responseItems}
}

func (p DreamTrackerPresenter) PresentSubmitRequirement(output service.SubmitDreamRequirementOutput) dto.SubmitDreamRequirementResponse {
	presentation := p.requirementStatusPolicy.Present(output.Requirement.Status)
	return dto.SubmitDreamRequirementResponse{
		DreamReqStatusID: output.Requirement.DreamReqStatusID,
		DocumentID:       output.Requirement.DocumentID,
		Status:           string(output.Requirement.Status),
		AIStatus:         output.Requirement.AIStatus,
		AIMessages:       output.AIMessages,
		StatusLabel:      presentation.Label,
		StatusVariant:    presentation.Variant,
		Message:          requirementMessage(output.Requirement.Notes, output.AIMessages),
		Meta:             toDreamRequirementReviewMetaResponse(output.ReviewMeta),
	}
}

func (p DreamTrackerPresenter) PresentUploadRequirementDocument(output service.UploadDreamRequirementDocumentOutput) dto.UploadDreamRequirementDocumentResponse {
	presentation := p.requirementStatusPolicy.Present(output.Requirement.Status)
	return dto.UploadDreamRequirementDocumentResponse{
		DreamReqStatusID: output.Requirement.DreamReqStatusID,
		Status:           string(output.Requirement.Status),
		StatusLabel:      presentation.Label,
		StatusVariant:    presentation.Variant,
		Document:         toDreamRequirementDocumentResponse(output.Document),
		Review:           toDreamRequirementReviewResponse(output.Review),
	}
}

func (p DreamTrackerPresenter) presentRequirements(items []model.DreamRequirementDetail) []dto.DreamRequirementStatusResponse {
	requirements := make([]dto.DreamRequirementStatusResponse, 0, len(items))
	for _, item := range items {
		aiMessages := decodeAIMessages(item.AIMessages)
		presentation := p.requirementStatusPolicy.Present(item.Status)

		requirements = append(requirements, dto.DreamRequirementStatusResponse{
			DreamReqStatusID: item.DreamReqStatusID,
			DocumentID:       item.DocumentID,
			ReqCatalogID:     item.ReqCatalogID,
			RequirementKey:   item.RequirementKey,
			RequirementLabel: item.RequirementLabel,
			Label:            item.RequirementLabel,
			Category:         item.RequirementCategory,
			Description:      item.RequirementDescription,
			IsRequired:       item.IsRequired,
			Status:           string(item.Status),
			Notes:            item.Notes,
			AIStatus:         item.AIStatus,
			AIMessages:       aiMessages,
			StatusLabel:      presentation.Label,
			StatusVariant:    presentation.Variant,
			Message:          requirementMessage(item.Notes, aiMessages),
			ActionLabel:      item.ActionLabel,
			CanUpload:        item.CanUpload,
			NeedsReupload:    item.NeedsReupload,
			Document:         toDreamRequirementDocumentResponse(item.Document),
			Review:           toDreamRequirementReviewResponse(buildRequirementReview(item)),
			CreatedAt:        item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return requirements
}

type StatusPresentation struct {
	Label   string
	Variant string
}

type TrackerStatusPolicy interface {
	Present(detail repository.DreamTrackerDetail) StatusPresentation
}

type RequirementStatusPolicy interface {
	Present(status model.DreamRequirementStatusValue) StatusPresentation
}

type DefaultTrackerStatusPolicy struct{}

func (DefaultTrackerStatusPolicy) Present(detail repository.DreamTrackerDetail) StatusPresentation {
	switch detail.DreamTracker.Status {
	case model.DreamTrackerStatusCompleted:
		return StatusPresentation{Label: "Selesai", Variant: "SUCCESS"}
	case model.DreamTrackerStatusArchived:
		return StatusPresentation{Label: "Tertunda", Variant: "MUTED"}
	default:
		if detail.Summary.IsOverdue {
			return StatusPresentation{Label: "Deadline Terlewat", Variant: "WARNING"}
		}
		if detail.Summary.IsDeadlineNear {
			return StatusPresentation{Label: "Deadline Mendekat", Variant: "WARNING"}
		}
		return StatusPresentation{Label: "Sedang Diproses", Variant: "IN_PROGRESS"}
	}
}

type DefaultRequirementStatusPolicy struct{}

func (DefaultRequirementStatusPolicy) Present(status model.DreamRequirementStatusValue) StatusPresentation {
	switch status {
	case model.DreamRequirementStatusRejected:
		return StatusPresentation{Label: "Ditolak", Variant: "ERROR"}
	case model.DreamRequirementStatusNeedsReview:
		return StatusPresentation{Label: "Perlu verifikasi ulang", Variant: "WARNING"}
	case model.DreamRequirementStatusReviewing:
		return StatusPresentation{Label: "Sedang diperiksa", Variant: "IN_PROGRESS"}
	case model.DreamRequirementStatusReused:
		return StatusPresentation{Label: "Sudah tersedia", Variant: "SUCCESS"}
	case model.DreamRequirementStatusVerifiedWithWarning:
		return StatusPresentation{Label: "Perlu verifikasi ulang", Variant: "WARNING"}
	case model.DreamRequirementStatusUploaded, model.DreamRequirementStatusVerified:
		return StatusPresentation{Label: "Berhasil diunggah", Variant: "SUCCESS"}
	default:
		return StatusPresentation{Label: "Belum diunggah", Variant: "DEFAULT"}
	}
}

func toDreamTrackerSummaryResponse(summary model.DreamTrackerSummary) dto.DreamTrackerSummaryResponse {
	var nextDeadlineAt *string
	if summary.NextDeadlineAt != nil {
		value := summary.NextDeadlineAt.UTC().Format(time.RFC3339)
		nextDeadlineAt = &value
	}
	return dto.DreamTrackerSummaryResponse{
		CompletionPercentage:  summary.CompletionPercentage,
		CompletedRequirements: summary.CompletedRequirements,
		TotalRequirements:     summary.TotalRequirements,
		NextDeadlineAt:        nextDeadlineAt,
		IsDeadlineNear:        summary.IsDeadlineNear,
		IsOverdue:             summary.IsOverdue,
	}
}

func toDreamTrackerProgramInfoResponse(info model.DreamTrackerProgramInfo) dto.DreamTrackerProgramInfoResponse {
	var admissionDeadline *string
	if info.AdmissionDeadline != nil {
		value := info.AdmissionDeadline.UTC().Format(time.RFC3339)
		admissionDeadline = &value
	}
	return dto.DreamTrackerProgramInfoResponse{
		ProgramID:         info.ProgramID,
		ProgramName:       info.ProgramName,
		UniversityName:    info.UniversityName,
		AdmissionName:     info.AdmissionName,
		Intake:            info.Intake,
		AdmissionURL:      info.AdmissionURL,
		AdmissionDeadline: admissionDeadline,
	}
}

func toDreamTrackerMilestoneResponses(items []model.DreamKeyMilestone) []dto.DreamTrackerMilestoneResponse {
	milestones := make([]dto.DreamTrackerMilestoneResponse, 0, len(items))
	for _, item := range items {
		var deadlineDate *string
		if item.DeadlineDate != nil {
			value := item.DeadlineDate.UTC().Format(time.RFC3339)
			deadlineDate = &value
		}
		milestones = append(milestones, dto.DreamTrackerMilestoneResponse{
			DreamMilestoneID: item.DreamMilestoneID,
			Title:            item.Title,
			Description:      item.Description,
			DeadlineDate:     deadlineDate,
			IsRequired:       item.IsRequired,
			Status:           string(item.Status),
			CreatedAt:        item.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:        item.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	return milestones
}

func toDreamTrackerFundingResponses(items []model.DreamTrackerFundingOption) []dto.DreamTrackerFundingResponse {
	fundings := make([]dto.DreamTrackerFundingResponse, 0, len(items))
	for _, item := range items {
		fundings = append(fundings, dto.DreamTrackerFundingResponse{
			FundingID:      item.FundingID,
			NamaBeasiswa:   item.NamaBeasiswa,
			Deskripsi:      item.Deskripsi,
			Provider:       item.Provider,
			TipePembiayaan: string(item.TipePembiayaan),
			Website:        item.Website,
			Status:         string(item.Status),
		})
	}
	return fundings
}

func decodeAIMessages(raw *string) []string {
	if raw == nil || *raw == "" {
		return []string{}
	}

	var messages []string
	if err := json.Unmarshal([]byte(*raw), &messages); err == nil {
		return messages
	}

	return []string{*raw}
}

func requirementMessage(notes *string, aiMessages []string) *string {
	if notes != nil && *notes != "" {
		return notes
	}
	if len(aiMessages) > 0 && aiMessages[0] != "" {
		value := aiMessages[0]
		return &value
	}
	return nil
}

func dreamTrackerSubtitle(detail repository.DreamTrackerDetail) *string {
	for _, value := range []*string{detail.ProgramInfo.AdmissionName, detail.ProgramInfo.Intake, detail.ProgramInfo.ProgramName} {
		if value != nil && *value != "" {
			return value
		}
	}
	if len(detail.Fundings) > 0 && detail.Fundings[0].NamaBeasiswa != "" {
		value := detail.Fundings[0].NamaBeasiswa
		return &value
	}
	return nil
}

func toDreamRequirementReviewMetaResponse(meta *dto.DreamRequirementReviewMeta) *dto.DreamRequirementReviewMetaResponse {
	if meta == nil {
		return nil
	}
	checks := make([]dto.DreamRequirementValidationCheckResponse, 0, len(meta.ValidationChecks))
	for _, check := range meta.ValidationChecks {
		checks = append(checks, dto.DreamRequirementValidationCheckResponse{
			Field:  check.Field,
			Status: check.Status,
			Reason: check.Reason,
		})
	}
	return &dto.DreamRequirementReviewMetaResponse{
		DocumentType:       meta.DocumentType,
		VerificationStatus: meta.VerificationStatus,
		ConfidenceScore:    meta.ConfidenceScore,
		UserMessage:        meta.UserMessage,
		ValidationChecks:   checks,
	}
}

func toDreamRequirementDocumentResponse(doc *model.Document) *dto.DreamRequirementDocumentResponse {
	if doc == nil {
		return nil
	}
	return &dto.DreamRequirementDocumentResponse{
		DocumentID:       doc.DocumentID,
		OriginalFilename: doc.OriginalFilename,
		PublicURL:        doc.PublicURL,
		MIMEType:         doc.MIMEType,
		SizeBytes:        doc.SizeBytes,
		DocumentType:     string(doc.DocumentType),
		UploadedAt:       doc.UploadedAt.UTC().Format(time.RFC3339),
	}
}

func buildRequirementReview(item model.DreamRequirementDetail) model.DreamRequirementReview {
	review := model.DreamRequirementReview{
		Source:          "NEW_UPLOAD",
		Status:          "NOT_STARTED",
		LastProcessedAt: nil,
	}

	if item.Status == model.DreamRequirementStatusReused {
		review.Source = "REUSED_EXISTING"
		review.Status = "SKIPPED"
		review.IsReused = true
		review.IsAlreadyVerified = true
	}

	if item.Document != nil && !item.Document.UploadedAt.IsZero() {
		value := item.Document.UploadedAt
		review.LastProcessedAt = &value
	}

	switch strings.ToUpper(strings.TrimSpace(valueOrEmpty(item.AIStatus))) {
	case "FAILED":
		review.Status = "FAILED"
	case "PENDING":
		review.Status = "PENDING"
	case "PROCESSING":
		review.Status = "PROCESSING"
	case "COMPLETED", "SUCCESS":
		review.Status = "COMPLETED"
	}

	if item.Status == model.DreamRequirementStatusUploaded && strings.TrimSpace(valueOrEmpty(item.AIStatus)) == "" {
		review.Status = "COMPLETED"
	}
	if item.Status == model.DreamRequirementStatusReviewing {
		review.Status = "PROCESSING"
	}
	if item.Status == model.DreamRequirementStatusVerified || item.Status == model.DreamRequirementStatusVerifiedWithWarning {
		review.IsAlreadyVerified = true
		if review.Status == "NOT_STARTED" || review.Status == "PENDING" {
			review.Status = "COMPLETED"
		}
	}
	if item.Status == model.DreamRequirementStatusNotUploaded {
		review.Status = "NOT_STARTED"
		review.LastProcessedAt = nil
	}

	if messages := decodeAIMessages(item.AIMessages); len(messages) > 0 && strings.TrimSpace(messages[0]) != "" {
		review.AIMessage = &messages[0]
	} else if item.Notes != nil && strings.TrimSpace(*item.Notes) != "" {
		review.AIMessage = item.Notes
	}

	return review
}

func toDreamRequirementReviewResponse(review model.DreamRequirementReview) dto.DreamRequirementReviewStateResponse {
	var lastProcessedAt *string
	if review.LastProcessedAt != nil {
		value := review.LastProcessedAt.UTC().Format(time.RFC3339)
		lastProcessedAt = &value
	}
	return dto.DreamRequirementReviewStateResponse{
		Source:            review.Source,
		Status:            review.Status,
		IsReused:          review.IsReused,
		IsAlreadyVerified: review.IsAlreadyVerified,
		AIMessage:         review.AIMessage,
		LastProcessedAt:   lastProcessedAt,
	}
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
