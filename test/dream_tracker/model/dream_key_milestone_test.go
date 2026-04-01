package model_test

import (
	"testing"
	"time"

	"boundless-be/model"
)

func TestDreamKeyMilestoneStatusValuesModel(t *testing.T) {
	if model.DreamKeyMilestoneStatusNotStarted != "NOT_STARTED" {
		t.Fatalf("unexpected not started status: %q", model.DreamKeyMilestoneStatusNotStarted)
	}
	if model.DreamKeyMilestoneStatusDone != "DONE" {
		t.Fatalf("unexpected done status: %q", model.DreamKeyMilestoneStatusDone)
	}
	if model.DreamKeyMilestoneStatusMissed != "MISSED" {
		t.Fatalf("unexpected missed status: %q", model.DreamKeyMilestoneStatusMissed)
	}
}

func TestDreamKeyMilestoneModel(t *testing.T) {
	description := "Submission deadline"
	deadline := time.Date(2026, time.May, 20, 0, 0, 0, 0, time.UTC)
	now := time.Now().UTC()

	milestone := model.DreamKeyMilestone{
		DreamMilestoneID: "milestone-1",
		DreamTrackerID:   "tracker-1",
		Title:            "Submission Deadline",
		Description:      &description,
		DeadlineDate:     &deadline,
		IsRequired:       true,
		Status:           model.DreamKeyMilestoneStatusNotStarted,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if milestone.DreamMilestoneID != "milestone-1" {
		t.Fatalf("unexpected milestone id: %q", milestone.DreamMilestoneID)
	}
	if milestone.Description == nil || *milestone.Description != description {
		t.Fatal("expected description to be set")
	}
	if milestone.DeadlineDate == nil || !milestone.DeadlineDate.Equal(deadline) {
		t.Fatal("expected deadline date to be set")
	}
	if milestone.Status != model.DreamKeyMilestoneStatusNotStarted {
		t.Fatalf("unexpected milestone status: %q", milestone.Status)
	}
}
