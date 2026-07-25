package auth

import (
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

// Token validity decisions (issue #586 — Rule 12: models hold data, not
// decisions). Expiry/consumability are wall-clock policy decisions, so they
// live in the service layer with the clock injected instead of calling
// time.Now() inside the model. The model rows only hold the expiry/used
// timestamps; these helpers interpret them.

// InvitationTokenExpired reports whether a staff invitation has passed its
// expiry relative to now.
func InvitationTokenExpired(t *auth.InvitationToken, now time.Time) bool {
	return t != nil && now.After(t.ExpiresAt)
}

// GuardianInvitationExpired reports whether a guardian invitation has passed
// its expiry relative to now.
func GuardianInvitationExpired(i *auth.GuardianInvitation, now time.Time) bool {
	return i != nil && now.After(i.ExpiresAt)
}

// GuardianInvitationValid reports whether a guardian invitation can still be
// consumed: not revoked, not expired, and not already accepted.
func GuardianInvitationValid(i *auth.GuardianInvitation, now time.Time) bool {
	return i != nil && i.RevokedAt == nil && !GuardianInvitationExpired(i, now) && !i.IsAccepted()
}
