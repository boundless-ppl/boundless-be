package model

import "time"

type UserSubscription struct {
	UserSubscriptionID string
	UserID             string
	SubscriptionID     string
	SourcePaymentID    string

	PackageNameSnapshot    string
	DurationMonthsSnapshot int
	PriceAmountSnapshot    int64

	StartDate time.Time
	EndDate   time.Time

	CreatedAt time.Time
}
