package dto

type DreamRequirementReviewRequest struct {
	DreamReqStatusID string `json:"dream_req_status_id"`
	DreamTrackerID   string `json:"dream_tracker_id"`
	ReqCatalogID     string `json:"req_catalog_id"`
	DocumentID       string `json:"document_id"`
	DocumentURL      string `json:"document_url,omitempty"`
	MIMEType         string `json:"mime_type,omitempty"`
}

type DreamRequirementReviewResponse struct {
	Status     string   `json:"status"`
	AIStatus   string   `json:"ai_status"`
	AIMessages []string `json:"ai_messages"`
}
