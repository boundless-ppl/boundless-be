package model

type FundingType string

const (
	FundingTypeScholarship   FundingType = "SCHOLARSHIP"
	FundingTypeSelfFunded    FundingType = "SELF_FUNDED"
	FundingTypeAssistantship FundingType = "ASSISTANTSHIP"
	FundingTypeLoan          FundingType = "LOAN"
	FundingTypeSponsorship   FundingType = "SPONSORSHIP"
)

type FundingOption struct {
	FundingID      string
	NamaBeasiswa   string
	Deskripsi      *string
	Provider       string
	TipePembiayaan FundingType
	Website        string
}

type RequirementCatalog struct {
	ReqCatalogID string
	Key          string
	Label        string
	Kategori     string
	Deskripsi    *string
}

type BenefitCatalog struct {
	BenefitID string
	Key       string
	Label     string
	Kategori  string
	Deskripsi *string
}

type FundingRequirement struct {
	FundingReqID string
	FundingID    string
	ReqCatalogID string
	IsRequired   bool
	SortOrder    int
}

type FundingBenefit struct {
	FundingBenefitID string
	FundingID        string
	BenefitID        string
	ValueText        string
	SortOrder        int
}
