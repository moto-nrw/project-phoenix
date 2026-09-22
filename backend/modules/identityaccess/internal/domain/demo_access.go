package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

// DemoAccessLifetime is how long a demo access opens the demo (#3462).
const DemoAccessLifetime = 14 * 24 * time.Hour

// DemoAccessCooldown is how long an address waits for its next link (#3465).
// Within it a repeated request stores nothing and mails nothing, so nobody
// floods a foreign inbox through the public form.
const DemoAccessCooldown = 10 * time.Minute

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
	// ErrDemoCapacityReached reports that no further demo school may be
	// created (#3466).
	ErrDemoCapacityReached = errors.New("demo capacity reached")
)

// Limits of the public request (#3466). Visitors of a fair share one WLAN
// address, so the IP limit is far above the address limit.
const (
	DemoRequestWindow        = time.Hour
	DemoRequestsPerAddress   = 3
	DemoRequestsPerIPAddress = 60
)

// DemoAccessRateLimitedError reports a request over a limit and when the
// caller may ask again.
type DemoAccessRateLimitedError struct{ RetryAt time.Time }

func (e *DemoAccessRateLimitedError) Error() string { return "too many demo access requests" }

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
	CreatedAt    time.Time
	// SchoolSlug is the demo school this access enters (#3463).
	SchoolSlug string
	// Role is the demo role a request preselects (#3467). It is not stored:
	// it rides in the mailed link, and the entry page skips the role cards.
	Role DemoRole
}

// DemoRole is what a visitor of the public demo sees the demo school as
// (#3467). The visitor has one account; a switch changes its role.
type DemoRole string

const (
	DemoRoleCaregiver DemoRole = "caregiver"
	DemoRoleLead      DemoRole = "lead"
	DemoRoleAll       DemoRole = "all"
	// DemoRoleParent signs the visitor in to the parents portal as the
	// school's parent who carries the visitor's name (#3468).
	DemoRoleParent DemoRole = "parent"
)

// ParseDemoRole accepts the empty role, which chooses none.
func ParseDemoRole(value string) (DemoRole, error) {
	switch role := DemoRole(strings.TrimSpace(value)); role {
	case "", DemoRoleCaregiver, DemoRoleLead, DemoRoleAll, DemoRoleParent:
		return role, nil
	default:
		return "", ErrDemoAccessInvalid
	}
}

// SchoolRole is the system role the visitor's account holds in the role.
// Until the reduced permission sets exist, the caregiver is a standard staff
// member and the OGS lead an administrator, like "all functions".
func (r DemoRole) SchoolRole() string {
	if r == DemoRoleCaregiver {
		return "user"
	}
	return "admin"
}

// Progress of the demo school behind a demo access (#3463).
const (
	DemoSchoolPreparing = "preparing"
	DemoSchoolReady     = "ready"
	DemoSchoolFailed    = "failed"
)

// DemoSchoolEntry is what a demo access needs to enter its school. AccountID
// is the prospect's own caregiver; zero signs in the school's administrator.
type DemoSchoolEntry struct {
	Status    string
	TenantID  int64
	AccountID int64
	// ParentAccountID is the parent carrying the visitor's name (#3468);
	// zero in a school without one.
	ParentAccountID int64
	// Shared marks the administrator every visitor of the standing school
	// signs in as; no demo role switch may change its role (#3467).
	Shared bool
}

// DemoEntry is a redeemed demo access: the session and what the demo
// banner shows and reports about it (#3467).
type DemoEntry struct {
	AccessToken  string
	RefreshToken string
	AccessID     int64
	// Role is the demo role the session really has; empty when the caller
	// chose none and the account kept its role.
	Role   DemoRole
	Source string
	// FixedRole marks the standing school's shared administrator, whose
	// role no switch changes; the banner then offers none.
	FixedRole bool
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
	if a.Role, err = ParseDemoRole(string(a.Role)); err != nil {
		return err
	}
	if a.PersonName == "" || len(a.PersonName) > 120 || a.SchoolName == "" || len(a.SchoolName) > 120 || len(a.Source) > 60 {
		return ErrDemoAccessInvalid
	}
	return nil
}

// CoolingDown reports whether the access is too young for a further link.
func (a DemoAccess) CoolingDown(now time.Time) bool {
	return now.Before(a.CreatedAt.Add(DemoAccessCooldown))
}

// Expired reports whether the access no longer opens the demo at now.
func (a DemoAccess) Expired(now time.Time) bool { return !now.Before(a.ExpiresAt) }
