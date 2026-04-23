package repository

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"boundless-be/model"
)

type fakeFundingScanner struct {
	values []any
	err    error
}

func (f *fakeFundingScanner) Scan(dest ...any) error {
	if f.err != nil {
		return f.err
	}
	if len(dest) != len(f.values) {
		return errors.New("destination length mismatch")
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = f.values[i].(string)
		case *sql.NullString:
			*d = f.values[i].(sql.NullString)
		case *model.FundingType:
			*d = f.values[i].(model.FundingType)
		default:
			return errors.New("unsupported destination type")
		}
	}
	return nil
}

func TestFundingStatusForTracker(t *testing.T) {
	selected := "fund-1"
	if got := fundingStatusForTracker("fund-1", &selected); got != model.DreamTrackerFundingStatusSelected {
		t.Fatalf("expected selected status, got %s", got)
	}
	if got := fundingStatusForTracker("fund-2", &selected); got != model.DreamTrackerFundingStatusAvailable {
		t.Fatalf("expected available status, got %s", got)
	}
}

func TestBuildRequirementAction(t *testing.T) {
	cases := []struct {
		status        model.DreamRequirementStatusValue
		label         string
		canUpload     bool
		needsReupload bool
	}{
		{model.DreamRequirementStatusRejected, "Upload Ulang", true, true},
		{model.DreamRequirementStatusNeedsReview, "Upload Ulang", true, true},
		{model.DreamRequirementStatusUploaded, "Upload Ulang", true, false},
		{model.DreamRequirementStatusReviewing, "Upload Ulang", true, false},
		{model.DreamRequirementStatusNotUploaded, "Upload", true, false},
		{model.DreamRequirementStatusVerified, "", false, false},
	}
	for _, tc := range cases {
		label, canUpload, needsReupload := buildRequirementAction(tc.status)
		if label != tc.label || canUpload != tc.canUpload || needsReupload != tc.needsReupload {
			t.Fatalf("status %s expected (%s,%v,%v), got (%s,%v,%v)",
				tc.status, tc.label, tc.canUpload, tc.needsReupload, label, canUpload, needsReupload)
		}
	}
}

func TestAssignNullHelpers(t *testing.T) {
	var tptr *time.Time
	assignNullTime(&tptr, sql.NullTime{})
	if tptr != nil {
		t.Fatalf("expected nil time pointer, got %#v", tptr)
	}
	now := time.Now().UTC()
	assignNullTime(&tptr, sql.NullTime{Time: now, Valid: true})
	if tptr == nil || !tptr.Equal(now) {
		t.Fatalf("expected assigned time %v, got %#v", now, tptr)
	}

	var sptr *string
	assignNullString(&sptr, sql.NullString{})
	if sptr != nil {
		t.Fatalf("expected nil string pointer, got %#v", sptr)
	}
	assignNullString(&sptr, sql.NullString{String: "abc", Valid: true})
	if sptr == nil || *sptr != "abc" {
		t.Fatalf("expected assigned string abc, got %#v", sptr)
	}
}

func TestScanDreamTrackerFunding(t *testing.T) {
	item, err := scanDreamTrackerFunding(&fakeFundingScanner{
		values: []any{
			"fund-1",
			"LPDP",
			sql.NullString{String: "desc", Valid: true},
			"Kemendikbud",
			model.FundingTypeScholarship,
			"https://example.com",
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if item.FundingID != "fund-1" || item.NamaBeasiswa != "LPDP" || item.Deskripsi == nil || *item.Deskripsi != "desc" {
		t.Fatalf("unexpected funding item: %#v", item)
	}

	_, err = scanDreamTrackerFunding(&fakeFundingScanner{err: errors.New("scan failed")})
	if err == nil {
		t.Fatal("expected error from scanner")
	}
}
