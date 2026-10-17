package care

import "time"

// GuardianProfile is the complete directory snapshot read by the workflow.
// It carries no persistence mapping or owner policy.
type GuardianProfile struct {
	ID                     int64
	TenantID               int64
	CreatedAt              time.Time
	UpdatedAt              time.Time
	FirstName              string
	LastName               string
	Email                  *string
	AddressStreet          *string
	AddressCity            *string
	AddressPostalCode      *string
	AccountID              *int64
	HasAccount             bool
	PreferredContactMethod string
	LanguagePreference     string
	PortalLocale           *string
	Notes                  *string
	PhoneNumbers           []*GuardianProfilePhone
}

type GuardianProfilePhone struct {
	ID                int64
	TenantID          int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	GuardianProfileID int64
	PhoneNumber       string
	PhoneType         string
	Label             *string
	IsPrimary         bool
	Priority          int
}

// HasPortalAccount fails safe when the account flag and relation have drifted.
func (p *GuardianProfile) HasPortalAccount() bool { return p.HasAccount || p.AccountID != nil }
