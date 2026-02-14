package domain

type AccountID string

type Account struct {
	ID       AccountID
	Name     string
	Metadata AccountMetadata
	Auth     Auth
	Usage    Usage
	Limits   AccountLimitSnapshots
}

type AccountMetadata struct {
	Provider  string
	Model     string
	SecretRef string
	PlanType  string
}
