package dto

import "mime/multipart"

type AIProfileRecommendationRequest struct {
	TranscriptFile *multipart.FileHeader
	CVFile         *multipart.FileHeader
	Preferences    RecommendationPreferenceInput
	AllowedCandidates []RecommendationAllowedCandidateInput
}

type AITranscriptRecommendationRequest struct {
	TranscriptFile *multipart.FileHeader
	Preferences    RecommendationPreferenceInput
	AllowedCandidates []RecommendationAllowedCandidateInput
}

type AICVRecommendationRequest struct {
	CVFile      *multipart.FileHeader
	Preferences RecommendationPreferenceInput
	AllowedCandidates []RecommendationAllowedCandidateInput
}

type GlobalMatchAIStudentProfileSummaryResponse struct {
	AcademicBackground string   `json:"academic_background"`
	ExperienceSummary  string   `json:"experience_summary"`
	Strengths          []string `json:"strengths"`
	ImprovementAreas   []string `json:"improvement_areas"`
	PreferredThemes    []string `json:"preferred_themes"`
	RawText            string   `json:"raw_text"`
}

type GlobalMatchAIScoreBreakdownResponse struct {
	AcademicFit         int `json:"academic_fit"`
	PreferenceMatch     int `json:"preference_match"`
	CurriculumRelevance int `json:"curriculum_relevance"`
	AdmissionChance     int `json:"admission_chance"`
}

type GlobalMatchAIScholarshipRecommendationResponse struct {
	ScholarshipName string `json:"scholarship_name"`
	CoverageSummary string `json:"coverage_summary"`
	Selectivity     string `json:"selectivity"`
	EligibilityHint string `json:"eligibility_hint"`
}

type GlobalMatchAIApplicationStrategyResponse struct {
	Ambitious      string `json:"ambitious"`
	Target         string `json:"target"`
	BalancedOption string `json:"balanced_option"`
}

type GlobalMatchAITopRecommendationResponse struct {
	Rank                       int                                              `json:"rank"`
	ProgramID                  string                                           `json:"program_id,omitempty"`
	UniversityName             string                                           `json:"university_name"`
	ProgramName                string                                           `json:"program_name"`
	Country                    string                                           `json:"country"`
	FitScore                   int                                              `json:"fit_score"`
	AdmissionChanceScore       int                                              `json:"admission_chance_score"`
	OverallRecommendationScore int                                              `json:"overall_recommendation_score"`
	FitLevel                   string                                           `json:"fit_level"`
	AdmissionDifficulty        string                                           `json:"admission_difficulty"`
	ScoreBreakdown             GlobalMatchAIScoreBreakdownResponse              `json:"score_breakdown"`
	Overview                   string                                           `json:"overview"`
	WhyThisUniversity          string                                           `json:"why_this_university"`
	WhyThisProgram             string                                           `json:"why_this_program"`
	PreferenceReasoning        []string                                         `json:"preference_reasoning"`
	MatchEvidence              []string                                         `json:"match_evidence"`
	ScholarshipRecommendations []GlobalMatchAIScholarshipRecommendationResponse `json:"scholarship_recommendations"`
	Pros                       []string                                         `json:"pros"`
	Cons                       []string                                         `json:"cons"`
}

type GlobalMatchAIRecommendationResponse struct {
	StudentProfileSummary GlobalMatchAIStudentProfileSummaryResponse `json:"student_profile_summary"`
	TopRecommendations    []GlobalMatchAITopRecommendationResponse   `json:"top_recommendations"`
	SelectionReasoning    string                                     `json:"selection_reasoning"`
	ApplicationStrategy   GlobalMatchAIApplicationStrategyResponse   `json:"application_strategy"`
	FinalNotes            []string                                   `json:"final_notes"`
}
