package model

import "time"

type DreamTrackerStatus string

const (
	DreamTrackerStatusActive    DreamTrackerStatus = "ACTIVE"
	DreamTrackerStatusCompleted DreamTrackerStatus = "COMPLETED"
	DreamTrackerStatusArchived  DreamTrackerStatus = "ARCHIVED"
)

type DreamRequirementStatusValue string

const (
	DreamRequirementStatusNotUploaded DreamRequirementStatusValue = "NOT_UPLOADED"
	DreamRequirementStatusUploaded    DreamRequirementStatusValue = "UPLOADED"
	DreamRequirementStatusVerified    DreamRequirementStatusValue = "VERIFIED"
	DreamRequirementStatusRejected    DreamRequirementStatusValue = "REJECTED"
)

type DreamTracker struct {
	DreamTrackerID    string
	UserID            string
	ProgramID         string
	AdmissionID       *string
	FundingID         *string
	Title             string
	Status            DreamTrackerStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
	SourceType        string
	ReqSubmissionID   *string
	SourceRecResultID *string
}

type DreamRequirementStatus struct {
	DreamReqStatusID string
	DreamTrackerID   string
	DocumentID       *string
	ReqCatalogID     string
	Status           DreamRequirementStatusValue
	Notes            *string
	AIStatus         *string
	AIMessages       *string
	CreatedAt        time.Time
}

type DreamTrackerSummary struct {
	CompletionPercentage  int
	CompletedRequirements int
	TotalRequirements     int
	NextDeadlineAt        *time.Time
	IsDeadlineNear        bool
	IsOverdue             bool
}

type DreamTrackerProgramInfo struct {
	ProgramID         string
	ProgramName       *string
	UniversityName    *string
	AdmissionName     *string
	Intake            *string
	AdmissionURL      *string
	AdmissionDeadline *time.Time
}

type DreamRequirementDetail struct {
	DreamRequirementStatus
	RequirementKey         string
	RequirementLabel       string
	RequirementCategory    string
	RequirementDescription *string
	ActionLabel            string
	CanUpload              bool
	NeedsReupload          bool
}

type DreamTrackerFundingStatus string

const (
	DreamTrackerFundingStatusAvailable DreamTrackerFundingStatus = "AVAILABLE"
	DreamTrackerFundingStatusSelected  DreamTrackerFundingStatus = "SELECTED"
)

type DreamTrackerFundingOption struct {
	FundingID      string
	NamaBeasiswa   string
	Deskripsi      *string
	Provider       string
	TipePembiayaan FundingType
	Website        string
	Status         DreamTrackerFundingStatus
}
