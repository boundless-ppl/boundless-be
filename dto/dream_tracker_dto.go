package dto

type CreateDreamTrackerRequest struct {
	ProgramID         string  `json:"program_id"`
	AdmissionID       *string `json:"admission_id"`
	FundingID         *string `json:"funding_id"`
	Title             string  `json:"title"`
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
	DreamReqStatusID string                              `json:"dream_req_status_id"`
	DocumentID       *string                             `json:"document_id,omitempty"`
	Status           string                              `json:"status"`
	AIStatus         *string                             `json:"ai_status,omitempty"`
	AIMessages       []string                            `json:"ai_messages,omitempty"`
	StatusLabel      string                              `json:"status_label,omitempty"`
	StatusVariant    string                              `json:"status_variant,omitempty"`
	Message          *string                             `json:"message,omitempty"`
	Meta             *DreamRequirementReviewMetaResponse `json:"meta,omitempty"`
}

type UploadDreamRequirementDocumentResponse struct {
	DreamReqStatusID string                                 `json:"dream_req_status_id"`
	Status           string                                 `json:"status"`
	StatusLabel      string                                 `json:"status_label,omitempty"`
	StatusVariant    string                                 `json:"status_variant,omitempty"`
	Document         *DreamRequirementDocumentResponse      `json:"document"`
	Review           DreamRequirementReviewStateResponse    `json:"review"`
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
	Label            string   `json:"label,omitempty"`
	IsRequired       bool     `json:"is_required"`
	StatusLabel      string   `json:"status_label,omitempty"`
	StatusVariant    string   `json:"status_variant,omitempty"`
	Message          *string  `json:"message,omitempty"`
	ActionLabel      string   `json:"action_label,omitempty"`
	CanUpload        bool     `json:"can_upload"`
	NeedsReupload    bool     `json:"needs_reupload"`
	Document         *DreamRequirementDocumentResponse `json:"document"`
	Review           DreamRequirementReviewStateResponse `json:"review"`
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

type DreamTrackerProgressResponse struct {
	Percentage         int `json:"percentage"`
	CompletedDocuments int `json:"completed_documents"`
	TotalDocuments     int `json:"total_documents"`
}

type DreamTrackerResponse struct {
	DreamTrackerID    string                           `json:"dream_tracker_id"`
	UserID            string                           `json:"user_id"`
	ProgramID         string                           `json:"program_id"`
	AdmissionID       *string                          `json:"admission_id,omitempty"`
	FundingID         *string                          `json:"funding_id,omitempty"`
	Title             string                           `json:"title"`
	Subtitle          *string                          `json:"subtitle,omitempty"`
	Status            string                           `json:"status"`
	StatusLabel       string                           `json:"status_label,omitempty"`
	StatusVariant     string                           `json:"status_variant,omitempty"`
	CreatedAt         string                           `json:"created_at"`
	UpdatedAt         string                           `json:"updated_at"`
	SourceType        string                           `json:"source_type"`
	ReqSubmissionID   *string                          `json:"req_submission_id,omitempty"`
	SourceRecResultID *string                          `json:"source_rec_result_id,omitempty"`
	DeadlineAt        *string                          `json:"deadline_at,omitempty"`
	Progress          DreamTrackerProgressResponse     `json:"progress"`
	Summary           DreamTrackerSummaryResponse      `json:"summary"`
	Program           DreamTrackerProgramInfoResponse  `json:"program"`
	Requirements      []DreamRequirementStatusResponse `json:"requirements"`
	Milestones        []DreamTrackerMilestoneResponse  `json:"milestones"`
	Fundings          []DreamTrackerFundingResponse    `json:"fundings"`
}

type DreamTrackerListResponse struct {
	Items []DreamTrackerResponse `json:"items"`
}

type DreamTrackerDashboardSummaryResponse struct {
	TotalApplications int `json:"total_applications"`
	IncompleteCount   int `json:"incomplete_count"`
	CompletedCount    int `json:"completed_count"`
	DeadlineNearCount int `json:"deadline_near_count"`
}

type DreamTrackerGroupItemResponse struct {
	DreamTrackerID        string `json:"dream_tracker_id"`
	Title                 string `json:"title"`
	ProgramName           string `json:"program_name,omitempty"`
	AdmissionName         string `json:"admission_name,omitempty"`
	UniversityName        string `json:"university_name,omitempty"`
	Status                string `json:"status"`
	StatusLabel           string `json:"status_label,omitempty"`
	CompletionPercentage  int    `json:"completion_percentage"`
	IsSelected            bool   `json:"is_selected"`
}

type DreamTrackerUniversityGroupResponse struct {
	UniversityID   string                          `json:"university_id,omitempty"`
	UniversityName string                          `json:"university_name"`
	Items          []DreamTrackerGroupItemResponse `json:"items"`
}

type DreamTrackerFundingGroupResponse struct {
	FundingID   string                          `json:"funding_id,omitempty"`
	FundingName string                          `json:"funding_name"`
	Items       []DreamTrackerGroupItemResponse `json:"items"`
}

type DreamTrackerGroupedResponse struct {
	DefaultSelectedDreamTrackerID *string                              `json:"default_selected_dream_tracker_id,omitempty"`
	Universities                  []DreamTrackerUniversityGroupResponse `json:"universities"`
	Fundings                      []DreamTrackerFundingGroupResponse    `json:"fundings"`
	DefaultDetail                 *DreamTrackerResponse                 `json:"default_detail,omitempty"`
}

type DreamRequirementReviewMetaResponse struct {
	DocumentType       string                                    `json:"document_type,omitempty"`
	VerificationStatus string                                    `json:"verification_status,omitempty"`
	ConfidenceScore    float64                                   `json:"confidence_score,omitempty"`
	UserMessage        *string                                   `json:"user_message,omitempty"`
	ValidationChecks   []DreamRequirementValidationCheckResponse `json:"validation_checks,omitempty"`
}

type DreamRequirementValidationCheckResponse struct {
	Field  string `json:"field"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type DreamRequirementDocumentResponse struct {
	DocumentID       string `json:"document_id"`
	OriginalFilename string `json:"original_filename,omitempty"`
	PublicURL        string `json:"public_url,omitempty"`
	MIMEType         string `json:"mime_type"`
	SizeBytes        int64  `json:"size_bytes,omitempty"`
	DocumentType     string `json:"document_type,omitempty"`
	UploadedAt       string `json:"uploaded_at"`
}

type DreamRequirementReviewStateResponse struct {
	Source            string  `json:"source"`
	Status            string  `json:"status"`
	IsReused          bool    `json:"is_reused"`
	IsAlreadyVerified bool    `json:"is_already_verified"`
	AIMessage         *string `json:"ai_message,omitempty"`
	LastProcessedAt   *string `json:"last_processed_at,omitempty"`
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
