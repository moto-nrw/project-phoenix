package domain

import "time"

// TraegerAccount is an account at a Träger with its Träger roles (ADR 0084).
type TraegerAccount struct {
	AccountID int64
	Email     string
	Roles     []string
}

// TraegerInvitation is an invitation into a Träger-Büro (#4650). Token is the
// raw link token, known only to the call that created it; the store keeps
// its hash.
type TraegerInvitation struct {
	ID             int64
	OrganizationID int64
	Email          string
	Token          string
	Roles          []string
	ExpiresAt      time.Time
	UsedAt         *time.Time
	RevokedAt      *time.Time
	CreatedBy      *int64
	CreatedAt      time.Time
}

// Open reports an invitation that is neither used nor revoked.
func (i TraegerInvitation) Open() bool { return i.UsedAt == nil && i.RevokedAt == nil }
