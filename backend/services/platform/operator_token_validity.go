package platform

import (
	"time"

	"github.com/moto-nrw/project-phoenix/models/platform"
)

// Operator token/MFA validity decisions (issue #586 — Rule 12: models hold
// data, not decisions). Expiry/consumability for operator-portal tokens are
// wall-clock policy decisions, so they live in the service layer with the clock
// injected instead of calling time.Now() inside the model. The model rows only
// hold the expiry/used/revoked timestamps; these helpers interpret them.
//
// The production read paths already filter expiry/used/revoked in SQL
// (operator invitation FindValidByToken, OperatorMFAEmailChallenge
// FindActiveByOperatorID, OperatorMFATrustedDevice
// FindActiveByOperatorIDAndTokenHash); these helpers are the in-memory
// equivalents for non-DB decisions.

// OperatorInvitationTokenExpired reports whether an operator invitation token
// has passed its expiry relative to now.
func OperatorInvitationTokenExpired(t *platform.OperatorInvitationToken, now time.Time) bool {
	return t != nil && now.After(t.ExpiresAt)
}

// OperatorInvitationTokenValid reports whether an operator invitation token can
// still be consumed: not expired and not already used/revoked.
func OperatorInvitationTokenValid(t *platform.OperatorInvitationToken, now time.Time) bool {
	return t != nil && !OperatorInvitationTokenExpired(t, now) && !t.IsUsed()
}

// OperatorEmailChangeTokenExpired reports whether an operator email-change token
// has passed its expiry relative to now.
func OperatorEmailChangeTokenExpired(t *platform.OperatorEmailChangeToken, now time.Time) bool {
	return t != nil && now.After(t.Expiry)
}

// OperatorMFAEmailChallengeExpired reports whether an operator MFA email
// challenge code's TTL has passed relative to now.
func OperatorMFAEmailChallengeExpired(c *platform.OperatorMFAEmailChallenge, now time.Time) bool {
	return c != nil && now.After(c.ExpiresAt)
}

// OperatorMFATrustedDeviceExpired reports whether an operator trusted device's
// trust window has passed relative to now.
func OperatorMFATrustedDeviceExpired(d *platform.OperatorMFATrustedDevice, now time.Time) bool {
	return d != nil && now.After(d.ExpiresAt)
}

// OperatorMFATrustedDeviceActive is the trusted-device login bypass predicate:
// the device may skip MFA only if it is neither expired nor revoked.
func OperatorMFATrustedDeviceActive(d *platform.OperatorMFATrustedDevice, now time.Time) bool {
	return d != nil && !OperatorMFATrustedDeviceExpired(d, now) && !d.IsRevoked()
}
