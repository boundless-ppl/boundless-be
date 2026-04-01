package model_test

import (
	"testing"
	"time"

	"boundless-be/model"
)

func TestDreamTrackerModel(t *testing.T) {
	now := time.Now().UTC()
	admissionID := "admission-1"
	fundingID := "funding-1"
	reqSubmissionID := "submission-1"
	sourceRecResultID := "result-1"

	tracker := model.DreamTracker{
		DreamTrackerID:    "tracker-1",
		UserID:            "user-1",
		ProgramID:         "program-1",
		AdmissionID:       &admissionID,
		FundingID:         &fundingID,
		Title:             "My Japan Plan",
		Status:            "ACTIVE",
		CreatedAt:         now,
		UpdatedAt:         now,
		SourceType:        "RECOMMENDATION",
		ReqSubmissionID:   &reqSubmissionID,
		SourceRecResultID: &sourceRecResultID,
	}

	if tracker.DreamTrackerID != "tracker-1" {
		t.Fatalf("unexpected tracker id: %q", tracker.DreamTrackerID)
	}
	if tracker.AdmissionID == nil || *tracker.AdmissionID != admissionID {
		t.Fatal("expected admission id to be set")
	}
	if tracker.SourceRecResultID == nil || *tracker.SourceRecResultID != sourceRecResultID {
		t.Fatal("expected source recommendation result id to be set")
	}
}

func TestDreamRequirementStatusModel(t *testing.T) {
	now := time.Now().UTC()
	documentID := "document-1"
	notes := "Waiting for upload"
	aiStatus := "PENDING"
	aiMessages := "Need clearer transcript"

	status := model.DreamRequirementStatus{
		DreamReqStatusID: "dream-req-1",
		DreamTrackerID:   "tracker-1",
		DocumentID:       &documentID,
		ReqCatalogID:     "req-1",
		Status:           "NOT_STARTED",
		Notes:            &notes,
		AIStatus:         &aiStatus,
		AIMessages:       &aiMessages,
		CreatedAt:        now,
	}

	if status.DocumentID == nil || *status.DocumentID != documentID {
		t.Fatal("expected document id to be set")
	}
	if status.Notes == nil || *status.Notes != notes {
		t.Fatal("expected notes to be set")
	}
	if status.AIMessages == nil || *status.AIMessages != aiMessages {
		t.Fatal("expected ai messages to be set")
	}
}
