package dto

import "time"

type PreferenceInput struct {
	Key   string `json:"pref_key" binding:"required"`
	Value string `json:"pref_value" binding:"required"`
}

type CreateRecommendationSubmissionResponse struct {
	SubmissionID string `json:"submission_id"`
	Status       string `json:"status"`
	ResultSetID  string `json:"result_set_id,omitempty"`
}

type CreateRecommendationSubmissionRequest struct {
	TranscriptDocumentID *string           `json:"transcript_document_id"`
	CVDocumentID         *string           `json:"cv_document_id"`
	Preferences          []PreferenceInput `json:"preferences"`
}

type UploadRecommendationDocumentResponse struct {
	Document RecommendationDocumentResponse `json:"document"`
}

type AIPreference = PreferenceInput

type AIRecommendationRequest struct {
	SubmissionID  string         `json:"submission_id"`
	UserID        string         `json:"user_id"`
	TranscriptURL string         `json:"transcript_url,omitempty"`
	CVURL         string         `json:"cv_url,omitempty"`
	Preferences   []AIPreference `json:"preferences"`
}

type AIRecommendationResponse struct {
	GeneratedAt time.Time                      `json:"generated_at"`
	Results     []RecommendationResultResponse `json:"results"`
}

type RecommendationDocumentResponse struct {
	DocumentID       string `json:"document_id"`
	OriginalFilename string `json:"original_filename"`
	PublicURL        string `json:"public_url"`
	MIMEType         string `json:"mime_type"`
	SizeBytes        int64  `json:"size_bytes"`
	DocumentType     string `json:"document_type"`
	UploadedAt       string `json:"uploaded_at"`
}

type RecommendationPreferenceResponse struct {
	PrefKey   string `json:"pref_key"`
	PrefValue string `json:"pref_value"`
}

type RecommendationResultResponse struct {
	RecResultID        string   `json:"source_rec_result_id,omitempty"`
	ProgramID          string   `json:"program_id,omitempty"`
	RankNo            int      `json:"rank_no"`
	UniversityName    string   `json:"university_name"`
	ProgramName       string   `json:"program_name"`
	Country           string   `json:"country"`
	FitScore          int      `json:"fit_score"`
	FitLevel          string   `json:"fit_level"`
	Overview          string   `json:"overview"`
	WhyThisUniversity string   `json:"why_this_university"`
	WhyThisProgram    string   `json:"why_this_program"`
	ReasonSummary     string   `json:"reason_summary"`
	Pros              []string `json:"pros"`
	Cons              []string `json:"cons"`
}

type RecommendationResultSetResponse struct {
	ResultSetID string                         `json:"result_set_id"`
	VersionNo   int                            `json:"version_no"`
	GeneratedAt string                         `json:"generated_at"`
	Results     []RecommendationResultResponse `json:"results"`
}

type RecommendationSubmissionDetailResponse struct {
	SubmissionID string                             `json:"submission_id"`
	Status       string                             `json:"status"`
	CreatedAt    string                             `json:"created_at"`
	SubmittedAt  string                             `json:"submitted_at,omitempty"`
	Documents    []RecommendationDocumentResponse   `json:"documents"`
	Preferences  []RecommendationPreferenceResponse `json:"preferences"`
	LatestResult *RecommendationResultSetResponse   `json:"latest_result,omitempty"`
}

type RecommendationAllowedCandidateListResponse struct {
	Items []RecommendationAllowedCandidateInput `json:"items"`
}
