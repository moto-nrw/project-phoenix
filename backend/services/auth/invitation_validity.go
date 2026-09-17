package auth

import (
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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
// consumed: not expired and not already accepted.
func GuardianInvitationValid(i *auth.GuardianInvitation, now time.Time) bool {
	return i != nil && !GuardianInvitationExpired(i, now) && !i.IsAccepted()
}

// isNotFoundError reports the "no row" outcomes of the retained
// repositories the invitation and guardian flows still read through.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, userModels.ErrGuardianProfileNotFound) {
		return true
	}
	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		return errors.Is(dbErr.Err, sql.ErrNoRows)
	}
	return false
}
