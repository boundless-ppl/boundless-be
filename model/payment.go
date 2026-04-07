package model

import "time"

type PaymentStatus string

const (
	PaymentStatusPending PaymentStatus = "pending"
	PaymentStatusSuccess PaymentStatus = "success"
	PaymentStatusFailed  PaymentStatus = "failed"
)

type Payment struct {
	PaymentID      string
	TransactionID  string
	UserID         string
	SubscriptionID string

	PackageNameSnapshot    string
	DurationMonthsSnapshot int
	PriceAmountSnapshot    int64
	BenefitsSnapshot       []string

	PaymentChannel string
	QrisImageURL   string

	Status          PaymentStatus
	AdminNote       *string
	ProofDocumentID *string

	VerifiedBy *string
	VerifiedAt *time.Time
	PaidAt     *time.Time
	ExpiredAt  *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}
