package repository

import (
	"context"
	"time"

	"boundless-be/model"
)

type CreateRecommendationSubmissionParams struct {
	Submission model.RecommendationSubmission
	Preference model.RecommendationPreference
	Documents  []model.Document
}

type SaveRecommendationResultSetParams struct {
	ResultSet        model.RecommendationResultSet
	Results          []model.RecommendationResult
	SubmissionID     string
	CompletedAt      time.Time
	SubmissionStatus model.RecommendationStatus
}

type RecommendationSubmissionWriter interface {
	CreateRecommendationSubmission(ctx context.Context, params CreateRecommendationSubmissionParams) (model.RecommendationSubmission, error)
	UpdateRecommendationSubmissionStatus(ctx context.Context, submissionID, userID string, status model.RecommendationStatus, failureReason string, completedAt *time.Time) error
}

type RecommendationDocumentWriter interface {
	CreateDocument(ctx context.Context, doc model.Document) (model.Document, error)
}

type RecommendationResultWriter interface {
	SaveRecommendationResultSet(ctx context.Context, params SaveRecommendationResultSetParams) (model.RecommendationResultSet, error)
}
