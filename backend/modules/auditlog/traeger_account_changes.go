package auditlog

import (
	"context"
	"time"
)

// TraegerAccountChange is one change of the Träger-Büro account ledger
// (#4650, #5474): an invitation, its revocation or acceptance, a role change
// or a removal. Reads of the member list stay in the ledger but are no
// change.
type TraegerAccountChange struct {
	ID              int64
	OccurredAt      time.Time
	ActorAccountID  int64
	Action          string
	TargetAccountID *int64
	TargetEmail     *string
	RolesBefore     []string
	RolesAfter      []string
}

// TraegerAccountChangeQuery reads the changes of one Träger, newest first.
// It joins the caller's administrative transaction and never reaches the
// ledger of another Träger.
type TraegerAccountChangeQuery interface {
	// ListTraegerAccountChanges lists up to limit changes of the Träger
	// older than the entry beforeID; beforeID 0 starts at the newest.
	ListTraegerAccountChanges(ctx context.Context, organizationID, beforeID int64, limit int) ([]TraegerAccountChange, error)
}
