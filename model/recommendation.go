package model

import "time"

type DocumentType string

const (
	DocumentTypeTranscript DocumentType = "transcript"
	DocumentTypeCV         DocumentType = "cv"
)

type RecommendationStatus string

const (
	RecommendationStatusDraft      RecommendationStatus = "draft"
	RecommendationStatusProcessing RecommendationStatus = "processing"
	RecommendationStatusCompleted  RecommendationStatus = "completed"
	RecommendationStatusFailed     RecommendationStatus = "failed"
)

type Document struct {
	DocumentID       string
	UserID           string
	OriginalFilename string
	StoragePath      string
	PublicURL        string
	MIMEType         string
	SizeBytes        int64
	DocumentType     DocumentType
	UploadedAt       time.Time
}

type RecommendationSubmission struct {
	RecSubmissionID      string
	UserID               string
	TranscriptDocumentID *string
	CVDocumentID         *string
	Status               RecommendationStatus
	CreatedAt            time.Time
	SubmittedAt          *time.Time
}

type RecommendationPreference struct {
	PrefID          string
	RecSubmissionID string
	PreferenceKey   string
	PreferenceValue string
	CreatedAt       time.Time
}

type RecommendationResultSet struct {
	ResultSetID     string
	RecSubmissionID string
	VersionNo       int
	GeneratedAt     time.Time
	CreatedAt       time.Time
}

type RecommendationResult struct {
	RecResultID       string
	ResultSetID       string
	RankNo            int
	UniversityName    string
	ProgramName       string
	Country           string
	FitScore          int
	FitLevel          string
	Overview          string
	WhyThisUniversity string
	WhyThisProgram    string
	ReasonSummary     string
	ProsJSON          string
	ConsJSON          string
	CreatedAt         time.Time
}
