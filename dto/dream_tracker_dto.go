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
	RequirementKey   string   `json:"requirement_key,omitempty"`
	RequirementLabel string   `json:"requirement_label,omitempty"`
	Category         string   `json:"category,omitempty"`
	Description      *string  `json:"description,omitempty"`
	Status           string   `json:"status"`
	Notes            *string  `json:"notes,omitempty"`
	AIStatus         *string  `json:"ai_status,omitempty"`
	AIMessages       []string `json:"ai_messages,omitempty"`
	ActionLabel      string   `json:"action_label,omitempty"`
	CanUpload        bool     `json:"can_upload"`
	NeedsReupload    bool     `json:"needs_reupload"`
	CreatedAt        string   `json:"created_at"`
}

type DreamTrackerSummaryResponse struct {
	CompletionPercentage  int     `json:"completion_percentage"`
	CompletedRequirements int     `json:"completed_requirements"`
	TotalRequirements     int     `json:"total_requirements"`
	NextDeadlineAt        *string `json:"next_deadline_at,omitempty"`
	IsDeadlineNear        bool    `json:"is_deadline_near"`
	IsOverdue             bool    `json:"is_overdue"`
}

type DreamTrackerProgramInfoResponse struct {
	ProgramID         string  `json:"program_id"`
	ProgramName       *string `json:"program_name,omitempty"`
	UniversityName    *string `json:"university_name,omitempty"`
	AdmissionName     *string `json:"admission_name,omitempty"`
	Intake            *string `json:"intake,omitempty"`
	AdmissionURL      *string `json:"admission_url,omitempty"`
	AdmissionDeadline *string `json:"admission_deadline,omitempty"`
}

type DreamTrackerMilestoneResponse struct {
	DreamMilestoneID string  `json:"dream_milestone_id"`
	Title            string  `json:"title"`
	Description      *string `json:"description,omitempty"`
	DeadlineDate     *string `json:"deadline_date,omitempty"`
	IsRequired       bool    `json:"is_required"`
	Status           string  `json:"status"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

type DreamTrackerFundingResponse struct {
	FundingID      string  `json:"funding_id"`
	NamaBeasiswa   string  `json:"nama_beasiswa"`
	Deskripsi      *string `json:"deskripsi,omitempty"`
	Provider       string  `json:"provider"`
	TipePembiayaan string  `json:"tipe_pembiayaan"`
	Website        string  `json:"website"`
	Status         string  `json:"status"`
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
	Summary           DreamTrackerSummaryResponse      `json:"summary"`
	Program           DreamTrackerProgramInfoResponse  `json:"program"`
	Requirements      []DreamRequirementStatusResponse `json:"requirements"`
	Milestones        []DreamTrackerMilestoneResponse  `json:"milestones"`
	Fundings          []DreamTrackerFundingResponse    `json:"fundings"`
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
