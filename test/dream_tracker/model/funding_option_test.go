package model_test

import (
	"testing"

	"boundless-be/model"
)

func TestFundingTypeValuesModel(t *testing.T) {
	if model.FundingTypeScholarship != "SCHOLARSHIP" {
		t.Fatalf("unexpected scholarship funding type: %q", model.FundingTypeScholarship)
	}
	if model.FundingTypeSelfFunded != "SELF_FUNDED" {
		t.Fatalf("unexpected self funded type: %q", model.FundingTypeSelfFunded)
	}
	if model.FundingTypeAssistantship != "ASSISTANTSHIP" {
		t.Fatalf("unexpected assistantship type: %q", model.FundingTypeAssistantship)
	}
	if model.FundingTypeLoan != "LOAN" {
		t.Fatalf("unexpected loan type: %q", model.FundingTypeLoan)
	}
	if model.FundingTypeSponsorship != "SPONSORSHIP" {
		t.Fatalf("unexpected sponsorship type: %q", model.FundingTypeSponsorship)
	}
}

func TestFundingOptionModel(t *testing.T) {
	description := "Tuition support"

	option := model.FundingOption{
		FundingID:      "funding-1",
		NamaBeasiswa:   "LPDP",
		Deskripsi:      &description,
		Provider:       "Government",
		TipePembiayaan: model.FundingTypeScholarship,
		Website:        "https://example.com/funding",
	}

	if option.Deskripsi == nil || *option.Deskripsi != description {
		t.Fatal("expected description to be set")
	}
	if option.TipePembiayaan != model.FundingTypeScholarship {
		t.Fatalf("unexpected funding type: %q", option.TipePembiayaan)
	}
}

func TestRequirementCatalogModel(t *testing.T) {
	description := "Official academic transcript"
	item := model.RequirementCatalog{
		ReqCatalogID: "req-1",
		Key:          "transcript",
		Label:        "Transcript",
		Kategori:     "document",
		Deskripsi:    &description,
	}

	if item.Key != "transcript" {
		t.Fatalf("unexpected key: %q", item.Key)
	}
	if item.Deskripsi == nil || *item.Deskripsi != description {
		t.Fatal("expected description to be set")
	}
}

func TestBenefitCatalogModel(t *testing.T) {
	item := model.BenefitCatalog{
		BenefitID: "benefit-1",
		Key:       "tuition_waiver",
		Label:     "Tuition Waiver",
		Kategori:  "financial",
	}

	if item.BenefitID != "benefit-1" {
		t.Fatalf("unexpected benefit id: %q", item.BenefitID)
	}
	if item.Key != "tuition_waiver" {
		t.Fatalf("unexpected key: %q", item.Key)
	}
}

func TestFundingRequirementModel(t *testing.T) {
	item := model.FundingRequirement{
		FundingReqID: "funding-req-1",
		FundingID:    "funding-1",
		ReqCatalogID: "req-1",
		IsRequired:   true,
		SortOrder:    1,
	}

	if !item.IsRequired {
		t.Fatal("expected funding requirement to be required")
	}
	if item.SortOrder != 1 {
		t.Fatalf("unexpected sort order: %d", item.SortOrder)
	}
}

func TestFundingBenefitModel(t *testing.T) {
	item := model.FundingBenefit{
		FundingBenefitID: "funding-benefit-1",
		FundingID:        "funding-1",
		BenefitID:        "benefit-1",
		ValueText:        "Full tuition",
		SortOrder:        2,
	}

	if item.ValueText != "Full tuition" {
		t.Fatalf("unexpected value text: %q", item.ValueText)
	}
	if item.SortOrder != 2 {
		t.Fatalf("unexpected sort order: %d", item.SortOrder)
	}
}
