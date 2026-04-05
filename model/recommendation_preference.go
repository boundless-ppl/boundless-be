package model

import "time"

type RecommendationPreference struct {
	PrefID               string
	RecSubmissionID      string
	PreferenceKey        string
	PreferenceValue      string
	Continents           []string
	Countries            []string
	FieldsOfStudy        []string
	DegreeLevel          string
	Languages            []string
	BudgetPreferences    []string
	ScholarshipTypes     []string
	StartPeriods         []string
	AdditionalPreference string
	RawPreferenceJSON    string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
