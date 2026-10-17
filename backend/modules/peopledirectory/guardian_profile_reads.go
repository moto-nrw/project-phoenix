package peopledirectory

import (
	"context"
	"time"
)

// GuardianProfileSnapshot preserves every directory profile field, including
// an explicitly unset portal locale and any already-loaded phone relation.
type GuardianProfileSnapshot struct {
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
	PhoneNumbers           []*GuardianProfilePhoneSnapshot
}

type GuardianProfilePhoneSnapshot struct {
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
