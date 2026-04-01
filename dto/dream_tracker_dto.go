package dto

type CreateDreamTrackerRequest struct {
	ProgramID         string  `json:"program_id" binding:"required"`
	AdmissionID       *string `json:"admission_id"`
	FundingID         *string `json:"funding_id"`
	Title             string  `json:"title" binding:"required"`
	Status            string  `json:"status"`
	SourceType        string  `json:"source_type" binding:"required"`
	ReqSubmissionID   *string `json:"req_submission_id"`
	SourceRecResultID *string `json:"source_rec_result_id"`
}

type CreateDreamTrackerResponse struct {
	DreamTrackerID string `json:"dream_tracker_id"`
	Status         string `json:"status"`
}

type SubmitDreamRequirementRequest struct {
	DocumentID string `json:"document_id" binding:"required"`
}

type SubmitDreamRequirementResponse struct {
	DreamReqStatusID string   `json:"dream_req_status_id"`
	DocumentID       *string  `json:"document_id,omitempty"`
	Status           string   `json:"status"`
	AIStatus         *string  `json:"ai_status,omitempty"`
	AIMessages       []string `json:"ai_messages,omitempty"`
}

type DreamRequirementStatusResponse struct {
	DreamReqStatusID string   `json:"dream_req_status_id"`
	DocumentID       *string  `json:"document_id,omitempty"`
	ReqCatalogID     string   `json:"req_catalog_id"`
	Status           string   `json:"status"`
	Notes            *string  `json:"notes,omitempty"`
	AIStatus         *string  `json:"ai_status,omitempty"`
	AIMessages       []string `json:"ai_messages,omitempty"`
	CreatedAt        string   `json:"created_at"`
}

type DreamTrackerResponse struct {
	DreamTrackerID    string                           `json:"dream_tracker_id"`
	UserID            string                           `json:"user_id"`
	ProgramID         string                           `json:"program_id"`
	AdmissionID       *string                          `json:"admission_id,omitempty"`
	FundingID         *string                          `json:"funding_id,omitempty"`
	Title             string                           `json:"title"`
	Status            string                           `json:"status"`
	CreatedAt         string                           `json:"created_at"`
	UpdatedAt         string                           `json:"updated_at"`
	SourceType        string                           `json:"source_type"`
	ReqSubmissionID   *string                          `json:"req_submission_id,omitempty"`
	SourceRecResultID *string                          `json:"source_rec_result_id,omitempty"`
	Requirements      []DreamRequirementStatusResponse `json:"requirements"`
}

type DreamTrackerDocumentResponse struct {
	DocumentID       string `json:"document_id"`
	UserID           string `json:"user_id"`
	Nama             string `json:"nama,omitempty"`
	OriginalFilename string `json:"original_filename,omitempty"`
	DokumenURL       string `json:"dokumen_url,omitempty"`
	PublicURL        string `json:"public_url,omitempty"`
	MIMEType         string `json:"mime_type"`
	SizeBytes        int64  `json:"size_bytes,omitempty"`
	DokumenSizeKB    int64  `json:"dokumen_size_kb,omitempty"`
	DocumentType     string `json:"document_type,omitempty"`
	UploadedAt       string `json:"uploaded_at"`
}
