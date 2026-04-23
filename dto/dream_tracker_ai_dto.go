package dto

type DreamRequirementReviewRequest struct {
	DreamReqStatusID     string `json:"dream_req_status_id"`
	DreamTrackerID       string `json:"dream_tracker_id"`
	ReqCatalogID         string `json:"req_catalog_id"`
	DocumentID           string `json:"document_id"`
	DocumentURL          string `json:"document_url,omitempty"`
	MIMEType             string `json:"mime_type,omitempty"`
	RequiredDocumentType string `json:"required_document_type,omitempty"`
	RequirementLabel     string `json:"requirement_label,omitempty"`
	ReuploadReason       string `json:"reupload_reason,omitempty"`
}

type DreamRequirementReviewResponse struct {
	Status         string                      `json:"status"`
	AIStatus       string                      `json:"ai_status"`
	AIMessages     []string                    `json:"ai_messages"`
	Meta           *DreamRequirementReviewMeta `json:"meta,omitempty"`
	ReuploadReason string                      `json:"reupload_reason,omitempty"`
	CanReupload    bool                        `json:"can_reupload,omitempty"`
}

type DreamRequirementReviewMeta struct {
	DocumentType       string                            `json:"document_type,omitempty"`
	VerificationStatus string                            `json:"verification_status,omitempty"`
	ConfidenceScore    float64                           `json:"confidence_score,omitempty"`
	UserMessage        *string                           `json:"user_message,omitempty"`
	ValidationChecks   []DreamRequirementValidationCheck `json:"validation_checks,omitempty"`
}

type DreamRequirementValidationCheck struct {
	Field  string `json:"field"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}
