package domain

import "time"

type AccountID string

type Account struct {
	ID           AccountID
	Name         string
	Metadata     AccountMetadata
	Auth         Auth
	Usage        Usage
	Limits       AccountLimitSnapshots
	Subscription *Subscription
}

type AccountMetadata struct {
	Provider  string
	Model     string
	SecretRef string
	PlanType  string
}

type Subscription struct {
	ActiveStart     time.Time
	ActiveUntil     time.Time
	WillRenew       bool
	BillingPeriod   string
	BillingCurrency string
	IsDelinquent    bool
	CapturedAt      time.Time
}
