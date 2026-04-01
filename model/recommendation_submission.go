package model

import "time"

type RecommendationStatus string

const (
	RecommendationStatusDraft      RecommendationStatus = "draft"
	RecommendationStatusProcessing RecommendationStatus = "processing"
	RecommendationStatusCompleted  RecommendationStatus = "completed"
	RecommendationStatusFailed     RecommendationStatus = "failed"
)

type RecommendationMode string

const (
	RecommendationModeProfile    RecommendationMode = "profile"
	RecommendationModeTranscript RecommendationMode = "transcript"
	RecommendationModeCV         RecommendationMode = "cv"
)

type RecommendationSubmission struct {
	RecSubmissionID      string
	UserID               string
	Mode                 RecommendationMode
	TranskripDokumenID   *string
	TranscriptDocumentID *string
	CVDokumenID          *string
	CVDocumentID         *string
	Status               RecommendationStatus
	FailureReason        string
	CreatedAt            time.Time
	SubmittedAt          *time.Time
	CompletedAt          *time.Time
	UpdatedAt            time.Time
}
