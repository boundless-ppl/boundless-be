package model

import "time"

type AdmissionPath struct {
	AdmissionID        string
	ProgramID          string
	Nama               string
	Intake             string
	Deadline           *time.Time
	RequiresSupervisor bool
	WebsiteURL         string
}

type AdmissionFunding struct {
	AdmissionFundingID string
	AdmissionID        string
	FundingID          string
	LinkageType        string
}
