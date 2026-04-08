package dto

import "mime/multipart"

type RecommendationPreferenceInput struct {
	Continents           []string `form:"continents" json:"continents"`
	Countries            []string `form:"countries" json:"countries"`
	FieldsOfStudy        []string `form:"fields_of_study" json:"fields_of_study"`
	DegreeLevel          string   `form:"degree_level" json:"degree_level"`
	Languages            []string `form:"languages" json:"languages"`
	BudgetPreferences    []string `form:"budget_preferences" json:"budget_preferences"`
	ScholarshipTypes     []string `form:"scholarship_types" json:"scholarship_types"`
	StartPeriods         []string `form:"start_periods" json:"start_periods"`
	AdditionalPreference string   `form:"additional_preference" json:"additional_preference"`
}

type RecommendationAllowedCandidateInput struct {
	ProgramID            string   `json:"program_id"`
	ProgramName          string   `json:"program_name"`
	UniversityName       string   `json:"university_name"`
	Country              string   `json:"country"`
	DegreeLevel          string   `json:"degree_level,omitempty"`
	Language             string   `json:"language,omitempty"`
	FocusTags            []string `json:"focus_tags,omitempty"`
	FundingSummary       []string `json:"funding_summary,omitempty"`
	AdmissionDeadline    string   `json:"admission_deadline,omitempty"`
	OfficialProgramURL   string   `json:"official_program_url,omitempty"`
	OfficialUniversityURL string  `json:"official_university_url,omitempty"`
}

type CreateProfileRecommendationRequest struct {
	TranscriptFile *multipart.FileHeader `form:"transcript_file" binding:"required"`
	CVFile         *multipart.FileHeader `form:"cv_file" binding:"required"`
	RecommendationPreferenceInput
}

type CreateTranscriptRecommendationRequest struct {
	TranscriptFile *multipart.FileHeader `form:"transcript_file" binding:"required"`
	RecommendationPreferenceInput
}

type CreateCVRecommendationRequest struct {
	CVFile *multipart.FileHeader `form:"cv_file" binding:"required"`
	RecommendationPreferenceInput
}

type CreateRecommendationWorkflowResponse struct {
	SubmissionID string                               `json:"submission_id"`
	Status       string                               `json:"status"`
	ResultSetID  string                               `json:"result_set_id,omitempty"`
	Result       *GlobalMatchAIRecommendationResponse `json:"result,omitempty"`
}
