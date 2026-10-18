package peopledirectory

import (
	"context"
	"time"
)

// GuardianProfileSnapshot preserves every directory profile field, including
// an explicitly unset portal locale and any already-loaded phone relation.
type GuardianProfileSnapshot struct {
	ID                     int64                           `json:"id"`
	TenantID               int64                           `json:"tenant_id"`
	CreatedAt              time.Time                       `json:"created_at"`
	UpdatedAt              time.Time                       `json:"updated_at"`
	FirstName              string                          `json:"first_name"`
	LastName               string                          `json:"last_name"`
	Email                  *string                         `json:"email,omitempty"`
	AddressStreet          *string                         `json:"address_street,omitempty"`
	AddressCity            *string                         `json:"address_city,omitempty"`
	AddressPostalCode      *string                         `json:"address_postal_code,omitempty"`
	AccountID              *int64                          `json:"account_id,omitempty"`
	HasAccount             bool                            `json:"has_account"`
	PreferredContactMethod string                          `json:"preferred_contact_method"`
	LanguagePreference     string                          `json:"language_preference"`
	PortalLocale           *string                         `json:"portal_locale,omitempty"`
	Notes                  *string                         `json:"notes,omitempty"`
	PhoneNumbers           []*GuardianProfilePhoneSnapshot `json:"phone_numbers,omitempty"`
}

type GuardianProfilePhoneSnapshot struct {
	ID                int64     `json:"id"`
	TenantID          int64     `json:"tenant_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	GuardianProfileID int64     `json:"guardian_profile_id"`
	PhoneNumber       string    `json:"phone_number"`
	PhoneType         string    `json:"phone_type"`
	Label             *string   `json:"label,omitempty"`
	IsPrimary         bool      `json:"is_primary"`
	Priority          int       `json:"priority"`
}

// GuardianProfileReads joins the caller's transaction and retains the existing
// row locks. NormalizeGuardianProfile applies the retained canonical profile validation,
// including mutations performed before a validation error.
type GuardianProfileReads interface {
	GuardianProfileByID(context.Context, int64) (*GuardianProfileSnapshot, error)
	GuardianProfilesByIDs(context.Context, []int64) (map[int64]*GuardianProfileSnapshot, error)
	GuardianProfileByEmail(context.Context, string) (*GuardianProfileSnapshot, error)
	GuardianProfileByAccount(context.Context, int64) (*GuardianProfileSnapshot, error)
	ActiveGuardianPortalProfiles(context.Context, []int64) (map[int64]*GuardianProfileSnapshot, error)
	LockGuardianProfile(context.Context, int64) error
	NormalizeGuardianProfile(*GuardianProfileSnapshot) error
}
