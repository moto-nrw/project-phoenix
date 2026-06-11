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

// PasswordResetRateLimitWindow is the throttling window for password-reset
// requests (backend CLAUDE.md: three reset requests per hour). The repository
// enforces the same window in SQL; this is the in-memory equivalent. Extracted
// from a bare time.Hour literal inside the model (issue #586).
const PasswordResetRateLimitWindow = time.Hour

// PasswordResetRateLimitExpired reports whether the rate-limit window has
// elapsed for the given record relative to now.
func PasswordResetRateLimitExpired(m *auth.PasswordResetRateLimit, now time.Time) bool {
	return m != nil && m.WindowStart.Add(PasswordResetRateLimitWindow).Before(now)
}

// ResetPasswordResetRateLimit starts a fresh window on the record and seeds the
// attempt counter to 1 (the reset itself counts as the first attempt of the new
// window). now is injected so the decision is testable.
func ResetPasswordResetRateLimit(m *auth.PasswordResetRateLimit, now time.Time) {
	if m == nil {
		return
	}
	m.Attempts = 1
	m.WindowStart = now.UTC()
}

// TokenExpired reports whether an authentication token has passed its expiry
// relative to now.
func TokenExpired(t *auth.Token, now time.Time) bool {
	return t != nil && t.Expiry.Before(now)
}

// PasswordResetTokenExpired reports whether a password-reset token has passed
// its expiry relative to now.
func PasswordResetTokenExpired(t *auth.PasswordResetToken, now time.Time) bool {
	return t != nil && t.Expiry.Before(now)
}

// PasswordResetTokenValid reports whether a password-reset token can still be
// consumed: not expired and not already used.
func PasswordResetTokenValid(t *auth.PasswordResetToken, now time.Time) bool {
	return t != nil && !PasswordResetTokenExpired(t, now) && !t.Used
}

// InvitationTokenExpired reports whether a staff invitation has passed its
// expiry relative to now.
func InvitationTokenExpired(t *auth.InvitationToken, now time.Time) bool {
	return t != nil && now.After(t.ExpiresAt)
}

// InvitationTokenValid reports whether a staff invitation can still be
// consumed: not expired and not already used/revoked.
func InvitationTokenValid(t *auth.InvitationToken, now time.Time) bool {
	return t != nil && !InvitationTokenExpired(t, now) && !t.IsUsed()
}

// GuardianInvitationExpired reports whether a guardian invitation has passed
// its expiry relative to now.
func GuardianInvitationExpired(i *auth.GuardianInvitation, now time.Time) bool {
	return i != nil && now.After(i.ExpiresAt)
}

// GuardianInvitationValid reports whether a guardian invitation can still be
// consumed: not expired and not already accepted.
func GuardianInvitationValid(i *auth.GuardianInvitation, now time.Time) bool {
	return i != nil && !GuardianInvitationExpired(i, now) && !i.IsAccepted()
}

// MFAEmailChallengeExpired reports whether an emailed MFA code's TTL has passed
// relative to now. The production read path already filters expired challenges
// in SQL (FindActiveByAccountID); this helper is the in-memory equivalent for
// non-DB decisions.
func MFAEmailChallengeExpired(c *auth.MFAEmailChallenge, now time.Time) bool {
	return c != nil && now.After(c.ExpiresAt)
}

// MFATrustedDeviceExpired reports whether a trusted device's trust window has
// passed relative to now.
func MFATrustedDeviceExpired(d *auth.MFATrustedDevice, now time.Time) bool {
	return d != nil && now.After(d.ExpiresAt)
}

// MFATrustedDeviceActive is the trusted-device login bypass predicate: the
// device may skip MFA only if it is neither expired nor revoked. The
// production read path already filters in SQL
// (FindActiveByAccountTenantAndTokenHash); this is the in-memory equivalent.
func MFATrustedDeviceActive(d *auth.MFATrustedDevice, now time.Time) bool {
	return d != nil && !MFATrustedDeviceExpired(d, now) && !d.IsRevoked()
}
