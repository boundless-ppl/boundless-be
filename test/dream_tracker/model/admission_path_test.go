package model_test

import (
	"testing"
	"time"

	"boundless-be/model"
)

func TestAdmissionPathModel(t *testing.T) {
	deadline := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)

	admission := model.AdmissionPath{
		AdmissionID:        "admission-1",
		ProgramID:          "program-1",
		Nama:               "Regular Intake",
		Intake:             "Fall 2026",
		Deadline:           &deadline,
		RequiresSupervisor: true,
		WebsiteURL:         "https://example.com/admission",
	}

	if admission.AdmissionID != "admission-1" {
		t.Fatalf("unexpected admission id: %q", admission.AdmissionID)
	}
	if admission.Deadline == nil || !admission.Deadline.Equal(deadline) {
		t.Fatal("expected deadline to be set")
	}
	if !admission.RequiresSupervisor {
		t.Fatal("expected requires supervisor to be true")
	}
}

func TestAdmissionFundingModel(t *testing.T) {
	link := model.AdmissionFunding{
		AdmissionFundingID: "admission-funding-1",
		AdmissionID:        "admission-1",
		FundingID:          "funding-1",
		LinkageType:        "OPTIONAL",
	}

	if link.AdmissionFundingID != "admission-funding-1" {
		t.Fatalf("unexpected admission funding id: %q", link.AdmissionFundingID)
	}
	if link.LinkageType != "OPTIONAL" {
		t.Fatalf("unexpected linkage type: %q", link.LinkageType)
	}
}
