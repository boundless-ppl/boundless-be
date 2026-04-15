package model

import "time"

type DreamKeyMilestoneStatus string

const (
	DreamKeyMilestoneStatusNotStarted DreamKeyMilestoneStatus = "NOT_STARTED"
	DreamKeyMilestoneStatusDone       DreamKeyMilestoneStatus = "DONE"
	DreamKeyMilestoneStatusMissed     DreamKeyMilestoneStatus = "MISSED"
)

type DreamKeyMilestone struct {
	DreamMilestoneID string
	DreamTrackerID   string
	Title            string
	Description      *string
	DeadlineDate     *time.Time
	IsRequired       bool
	Status           DreamKeyMilestoneStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
