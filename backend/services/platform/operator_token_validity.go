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
// (operator invitation FindValidByToken); these helpers are the in-memory
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
