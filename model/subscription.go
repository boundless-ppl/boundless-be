package model

import "time"

type Subscription struct {
	SubscriptionID string
	PackageKey     string
	Name           string
	Description    string

	DurationMonths      int
	PriceAmount         int64
	NormalPriceAmount   int64
	DiscountPriceAmount int64
	Benefits            []string

	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
