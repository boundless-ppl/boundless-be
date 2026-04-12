package dto

type DreamTrackerDashboardSummaryResponse struct {
	TotalApplications int `json:"total_applications"`
	IncompleteCount   int `json:"incomplete_count"`
	CompletedCount    int `json:"completed_count"`
	DeadlineNearCount int `json:"deadline_near_count"`
}

type DreamTrackerSummaryDataResponse struct {
	CompletionPercentage  int     `json:"completion_percentage"`
	CompletedRequirements int     `json:"completed_requirements"`
	TotalRequirements     int     `json:"total_requirements"`
	NextDeadlineAt        *string `json:"next_deadline_at"`
	IsDeadlineNear        bool    `json:"is_deadline_near"`
	IsOverdue             bool    `json:"is_overdue"`
}

type DreamTrackerProgramResponse struct {
	ProgramID         string `json:"program_id"`
	ProgramName       string `json:"program_name"`
	UniversityName    string `json:"university_name"`
	AdmissionName     string `json:"admission_name"`
	Intake            string `json:"intake"`
	AdmissionURL      string `json:"admission_url"`
	AdmissionDeadline string `json:"admission_deadline"`
}

type DreamRequirementDocumentResponse struct {
	DocumentID       string `json:"document_id"`
	DocumentType     string `json:"document_type"`
	OriginalFilename string `json:"original_filename"`
	PublicURL        string `json:"public_url"`
	MIMEType         string `json:"mime_type,omitempty"`
	UploadedAt       string `json:"uploaded_at"`
}

type DreamRequirementReviewResponse struct {
	Source            string  `json:"source"`
	Status            string  `json:"status"`
	IsReused          bool    `json:"is_reused"`
	IsAlreadyVerified bool    `json:"is_already_verified"`
	AIMessage         *string `json:"ai_message"`
	LastProcessedAt   *string `json:"last_processed_at"`
}

type DreamRequirementResponse struct {
	DreamReqStatusID string                            `json:"dream_req_status_id"`
	ReqCatalogID     string                            `json:"req_catalog_id"`
	RequirementKey   string                            `json:"requirement_key"`
	RequirementLabel string                            `json:"requirement_label"`
	Category         string                            `json:"category"`
	Status           string                            `json:"status"`
	StatusLabel      string                            `json:"status_label"`
	StatusVariant    string                            `json:"status_variant"`
	CanUpload        bool                              `json:"can_upload"`
	NeedsReupload    bool                              `json:"needs_reupload"`
	Document         *DreamRequirementDocumentResponse `json:"document"`
	Review           DreamRequirementReviewResponse    `json:"review"`
}

type DreamMilestoneResponse struct {
	DreamMilestoneID string `json:"dream_milestone_id"`
	Title            string `json:"title"`
	Status           string `json:"status"`
	DeadlineDate     string `json:"deadline_date"`
}

type DreamFundingResponse struct {
	FundingID    string `json:"funding_id"`
	NamaBeasiswa string `json:"nama_beasiswa"`
	Provider     string `json:"provider"`
	Status       string `json:"status"`
}

type DreamTrackerItemResponse struct {
	DreamTrackerID string                          `json:"dream_tracker_id"`
	Title          string                          `json:"title"`
	Subtitle       string                          `json:"subtitle"`
	Status         string                          `json:"status"`
	StatusLabel    string                          `json:"status_label"`
	StatusVariant  string                          `json:"status_variant"`
	CreatedAt      string                          `json:"created_at"`
	UpdatedAt      string                          `json:"updated_at"`
	DeadlineAt     *string                         `json:"deadline_at"`
	Summary        DreamTrackerSummaryDataResponse `json:"summary"`
	Program        DreamTrackerProgramResponse     `json:"program"`
	Requirements   []DreamRequirementResponse      `json:"requirements"`
	Milestones     []DreamMilestoneResponse        `json:"milestones"`
	Fundings       []DreamFundingResponse          `json:"fundings"`
}

type DreamTrackerGroupedUniversityItemResponse struct {
	DreamTrackerID       string `json:"dream_tracker_id"`
	Title                string `json:"title"`
	ProgramName          string `json:"program_name"`
	AdmissionName        string `json:"admission_name"`
	Status               string `json:"status"`
	StatusLabel          string `json:"status_label"`
	CompletionPercentage int    `json:"completion_percentage"`
	IsSelected           bool   `json:"is_selected"`
}

type DreamTrackerGroupedUniversityResponse struct {
	UniversityID   string                                      `json:"university_id"`
	UniversityName string                                      `json:"university_name"`
	Items          []DreamTrackerGroupedUniversityItemResponse `json:"items"`
}

type DreamTrackerGroupedFundingItemResponse struct {
	DreamTrackerID       string `json:"dream_tracker_id"`
	Title                string `json:"title"`
	ProgramName          string `json:"program_name"`
	UniversityName       string `json:"university_name"`
	Status               string `json:"status"`
	StatusLabel          string `json:"status_label"`
	CompletionPercentage int    `json:"completion_percentage"`
	IsSelected           bool   `json:"is_selected"`
}

type DreamTrackerGroupedFundingResponse struct {
	FundingID   string                                   `json:"funding_id"`
	FundingName string                                   `json:"funding_name"`
	Items       []DreamTrackerGroupedFundingItemResponse `json:"items"`
}

type DreamTrackerGroupedResponse struct {
	DefaultSelectedDreamTrackerID string                                  `json:"default_selected_dream_tracker_id"`
	Universities                  []DreamTrackerGroupedUniversityResponse `json:"universities"`
	Fundings                      []DreamTrackerGroupedFundingResponse    `json:"fundings"`
	DefaultDetail                 *DreamTrackerItemResponse               `json:"default_detail,omitempty"`
}

type CreateDreamTrackerRequest struct {
	ProgramID         string  `json:"program_id" binding:"required"`
	AdmissionID       *string `json:"admission_id"`
	FundingID         *string `json:"funding_id"`
	Title             *string `json:"title"`
	Status            *string `json:"status"`
	SourceType        string  `json:"source_type" binding:"required"`
	ReqSubmissionID   *string `json:"req_submission_id"`
	SourceRecResultID *string `json:"source_rec_result_id"`
}

type CreateDreamTrackerResponse struct {
	DreamTrackerID string `json:"dream_tracker_id"`
	Status         string `json:"status"`
}

type SubmitRequirementResponse struct {
	DreamReqStatusID string                            `json:"dream_req_status_id"`
	Status           string                            `json:"status"`
	StatusLabel      string                            `json:"status_label"`
	StatusVariant    string                            `json:"status_variant"`
	Document         *DreamRequirementDocumentResponse `json:"document"`
	Review           DreamRequirementReviewResponse    `json:"review"`
}
