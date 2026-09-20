package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

// DemoAccessLifetime is how long a demo access opens the demo (#3462).
const DemoAccessLifetime = 14 * 24 * time.Hour

var (
	// ErrDemoAccessInvalid reports a request whose fields do not validate.
	ErrDemoAccessInvalid = errors.New("demo access request is invalid")
	// ErrDemoAccessUnknown reports a token no demo access was issued for.
	ErrDemoAccessUnknown = errors.New("demo access is unknown")
	// ErrDemoAccessExpired reports a demo access past its lifetime.
	ErrDemoAccessExpired = errors.New("demo access has expired")
	// ErrDemoSchoolPreparing reports a demo school nobody can enter yet.
	ErrDemoSchoolPreparing = errors.New("demo school is being prepared")
	// ErrDemoSessionTenantLocked reports a tenant switch of a demo session.
	ErrDemoSessionTenantLocked = errors.New("demo sessions cannot switch tenants")
)

// DemoAccess connects a prospect's address with the demo. Only the SHA-256
// fingerprint of its token is kept.
type DemoAccess struct {
	ID           int64
	Email        string
	PersonName   string
	SchoolName   string
	Source       string
	ContactOptIn bool
	TokenHash    string
	ExpiresAt    time.Time
}

// Normalize trims the prospect's fields and validates them.
func (a *DemoAccess) Normalize() error {
	a.Email = strings.ToLower(strings.TrimSpace(a.Email))
	a.PersonName = strings.TrimSpace(a.PersonName)
	a.SchoolName = strings.TrimSpace(a.SchoolName)
	a.Source = strings.TrimSpace(a.Source)
	parsed, err := mail.ParseAddress(a.Email)
	if err != nil || parsed.Address != a.Email || len(a.Email) > 254 {
		return ErrDemoAccessInvalid
	}
	if a.PersonName == "" || len(a.PersonName) > 120 || a.SchoolName == "" || len(a.SchoolName) > 120 || len(a.Source) > 60 {
		return ErrDemoAccessInvalid
	}
	return nil
}

// Expired reports whether the access no longer opens the demo at now.
func (a DemoAccess) Expired(now time.Time) bool { return !now.Before(a.ExpiresAt) }
