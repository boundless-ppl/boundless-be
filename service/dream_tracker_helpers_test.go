package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"boundless-be/model"
	"boundless-be/repository"
)

func TestUseLocalDocumentPathForAI(t *testing.T) {
	t.Setenv("AI_SERVICE_URL", "http://localhost:8000")
	if !useLocalDocumentPathForAI() {
		t.Fatal("expected localhost AI URL to use local document path")
	}

	t.Setenv("AI_SERVICE_URL", "https://ai.example.com")
	if useLocalDocumentPathForAI() {
		t.Fatal("expected non-local AI URL to not use local document path")
	}
}

func TestResolveLocalDocumentPathWithRelativePath(t *testing.T) {
	t.Setenv("DOCUMENT_STORAGE_DIR", "/tmp/boundless-storage")
	got := resolveLocalDocumentPath("user-1/PASSPORT/abc.pdf")
	if !strings.HasSuffix(got, "/tmp/boundless-storage/user-1/PASSPORT/abc.pdf") {
		t.Fatalf("unexpected resolved path: %s", got)
	}
}

func TestResolveLocalDocumentPathWithAbsolutePath(t *testing.T) {
	absolute := "/tmp/existing/path.pdf"
	got := resolveLocalDocumentPath(absolute)
	if got != absolute {
		t.Fatalf("expected absolute path to be unchanged, got %s", got)
	}
}

func TestNullableTrimmedStringHelpers(t *testing.T) {
	if nullableTrimmedString("   ") != nil {
		t.Fatal("expected nil for whitespace-only string")
	}
	value := nullableTrimmedString(" abc ")
	if value == nil || *value != " abc " {
		t.Fatalf("expected original value pointer, got %#v", value)
	}

	if nullableTrimmedStringPtr(nil) != nil {
		t.Fatal("expected nil for nil pointer")
	}
	blank := "   "
	if nullableTrimmedStringPtr(&blank) != nil {
		t.Fatal("expected nil for blank pointer value")
	}
	raw := "  hello  "
	trimmed := nullableTrimmedStringPtr(&raw)
	if trimmed == nil || *trimmed != "hello" {
		t.Fatalf("expected trimmed value hello, got %#v", trimmed)
	}
}

func TestCanonicalizeDreamRequirementDocumentType(t *testing.T) {
	cases := map[string]string{
		"passport":           "PASSPORT",
		"Paspor":             "PASSPORT",
		"transkrip nilai":    "TRANSCRIPT",
		"financial_proof":    "BANK_STATEMENT",
		"recommendation lor": "RECOMMENDATION_LETTER",
		"custom_type":        "custom_type",
	}
	for in, expected := range cases {
		if got := canonicalizeDreamRequirementDocumentType(in); got != expected {
			t.Fatalf("input %q expected %q got %q", in, expected, got)
		}
	}
	if got := canonicalizeDreamRequirementDocumentType("   "); got != "" {
		t.Fatalf("expected empty result for blank input, got %q", got)
	}
}

func TestUploadReviewStatus(t *testing.T) {
	pending := "PENDING"
	cases := []struct {
		status   model.DreamRequirementStatusValue
		aiStatus *string
		expected string
	}{
		{model.DreamRequirementStatusReviewing, nil, "PROCESSING"},
		{model.DreamRequirementStatusReused, nil, "SKIPPED"},
		{model.DreamRequirementStatusUploaded, &pending, "PENDING"},
		{model.DreamRequirementStatusUploaded, nil, "COMPLETED"},
		{model.DreamRequirementStatusVerified, nil, "COMPLETED"},
		{model.DreamRequirementStatusVerifiedWithWarning, nil, "COMPLETED"},
		{model.DreamRequirementStatusRejected, nil, "FAILED"},
		{model.DreamRequirementStatusNotUploaded, nil, "NOT_STARTED"},
	}
	for _, tc := range cases {
		got := uploadReviewStatus(model.DreamRequirementStatus{
			Status:   tc.status,
			AIStatus: tc.aiStatus,
		})
		if got != tc.expected {
			t.Fatalf("status %s expected %s got %s", tc.status, tc.expected, got)
		}
	}
}

func TestBuildUploadReviewAndMessageHelpers(t *testing.T) {
	now := time.Now().UTC()
	note := "manual note"
	review := buildUploadReview("NEW_UPLOAD", "COMPLETED", false, true, &note, now)
	if review.Source != "NEW_UPLOAD" || review.Status != "COMPLETED" || !review.IsAlreadyVerified {
		t.Fatalf("unexpected review payload: %#v", review)
	}
	if review.LastProcessedAt == nil {
		t.Fatal("expected processed timestamp")
	}

	if msg := selectRequirementMessage(&note, []string{"ai message"}); msg == nil || *msg != note {
		t.Fatalf("expected note to win, got %#v", msg)
	}
	if msg := selectRequirementMessage(nil, []string{"ai message"}); msg == nil || *msg != "ai message" {
		t.Fatalf("expected ai message fallback, got %#v", msg)
	}
	if msg := selectRequirementMessage(nil, nil); msg != nil {
		t.Fatalf("expected nil message, got %#v", msg)
	}
}

func TestCompletionAndDeadlineHelpers(t *testing.T) {
	now := time.Now().UTC()
	soon := now.Add(48 * time.Hour)
	later := now.Add(10 * 24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	requirements := []model.DreamRequirementDetail{
		{DreamRequirementStatus: model.DreamRequirementStatus{Status: model.DreamRequirementStatusUploaded}},
		{DreamRequirementStatus: model.DreamRequirementStatus{Status: model.DreamRequirementStatusVerified}},
		{DreamRequirementStatus: model.DreamRequirementStatus{Status: model.DreamRequirementStatusNotUploaded}},
	}
	milestones := []model.DreamKeyMilestone{
		{DeadlineDate: &later},
		{DeadlineDate: &soon},
	}

	summary := buildDreamTrackerSummary(requirements, milestones, &past)
	if summary.CompletedRequirements != 2 || summary.TotalRequirements != 3 {
		t.Fatalf("unexpected summary counts: %#v", summary)
	}
	if summary.CompletionPercentage != 66 {
		t.Fatalf("expected completion 66, got %d", summary.CompletionPercentage)
	}
	if summary.NextDeadlineAt == nil || summary.NextDeadlineAt.Before(now) {
		t.Fatalf("expected next upcoming deadline, got %#v", summary.NextDeadlineAt)
	}
	if !summary.IsDeadlineNear || !summary.IsOverdue {
		t.Fatalf("expected near deadline true and overdue true due to past admission deadline, got %#v", summary)
	}

	if calculateCompletionPercentage(0, 0) != 0 {
		t.Fatal("expected zero completion when total is zero")
	}
	if !isCompletedRequirement(model.DreamRequirementStatusVerifiedWithWarning) {
		t.Fatal("expected verified-with-warning treated as complete")
	}
	if isCompletedRequirement(model.DreamRequirementStatusRejected) {
		t.Fatal("expected rejected not treated as complete")
	}
}

func TestTrackerAndGroupItemHelpers(t *testing.T) {
	programName := "Computer Science"
	admission := "Fall 2026"
	univ := "University A"
	detail := repository.DreamTrackerDetail{
		DreamTracker: model.DreamTracker{
			DreamTrackerID: "tracker-1",
			Title:          "My Dream",
			Status:         model.DreamTrackerStatusActive,
		},
		ProgramInfo: model.DreamTrackerProgramInfo{
			ProgramName:    &programName,
			AdmissionName:  &admission,
			UniversityName: &univ,
		},
		Summary: model.DreamTrackerSummary{
			TotalRequirements:     2,
			CompletedRequirements: 2,
			CompletionPercentage:  100,
			IsOverdue:             true,
		},
	}

	if !isTrackerCompleted(detail) {
		t.Fatal("expected tracker completed when all requirements complete")
	}
	item := buildDreamTrackerGroupItem(detail, "tracker-1")
	if !item.IsSelected || item.StatusLabel != "Deadline Terlewat" {
		t.Fatalf("unexpected group item: %#v", item)
	}
}

func TestPDFAndUtilityHelpers(t *testing.T) {
	if !isPDFDocument(model.Document{MIMEType: "application/pdf"}) {
		t.Fatal("expected mime-based pdf detection")
	}
	if !isPDFDocument(model.Document{OriginalFilename: "resume.PDF"}) {
		t.Fatal("expected extension-based pdf detection")
	}
	if isPDFDocument(model.Document{OriginalFilename: "image.png", MIMEType: "image/png"}) {
		t.Fatal("expected non-pdf detection")
	}

	if got := firstNonEmpty(" ", "", "first", "second"); got != "first" {
		t.Fatalf("expected first non-empty value, got %q", got)
	}
	if got := valueOrEmptyString(nil); got != "" {
		t.Fatalf("expected empty for nil pointer, got %q", got)
	}
	raw := "abc"
	if got := valueOrEmptyString(&raw); got != "abc" {
		t.Fatalf("expected abc, got %q", got)
	}

	payload := mustMarshalMessages([]string{"a", "b"})
	var messages []string
	if err := json.Unmarshal([]byte(payload), &messages); err != nil {
		t.Fatalf("expected valid json payload, got %v", err)
	}
	if len(messages) != 2 || messages[0] != "a" || messages[1] != "b" {
		t.Fatalf("unexpected marshalled payload: %#v", messages)
	}
}
