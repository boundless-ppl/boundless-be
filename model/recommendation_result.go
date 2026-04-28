package model

import "time"

type RecommendationResultSet struct {
	ResultSetID               string
	RecSubmissionID           string
	VersionNo                 int
	GeneratedAt               time.Time
	StudentProfileSummaryJSON string
	SelectionReasoningJSON    string
	ApplicationStrategyJSON   string
	FinalNotesJSON            string
	RawResponseJSON           string
	CreatedAt                 time.Time
}

type RecommendationResult struct {
	RecResultID                    string
	ResultSetID                    string
	ProgramID                      *string
	AdmissionID                    *string
	RankNo                         int
	UniversityName                 string
	ProgramName                    string
	Country                        string
	FitScore                       int
	AdmissionChanceScore           int
	OverallRecommendationScore     int
	FitLevel                       string
	AdmissionDifficulty            string
	ScoreBreakdownJSON             string
	Overview                       string
	WhyThisUniversity              string
	WhyThisProgram                 string
	PreferenceReasoningJSON        string
	MatchEvidenceJSON              string
	ScholarshipRecommendationsJSON string
	ReasonSummary                  string
	ProsJSON                       string
	ConsJSON                       string
	RawRecommendationJSON          string
	CreatedAt                      time.Time
}
