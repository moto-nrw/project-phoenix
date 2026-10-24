package peopledirectory

import (
	"context"
	"time"
)

// RequestShareVisibility resolves a recorded request's effective recipients
// inside the caller's tenant transaction. It is not a guardian access grant.
type RequestShareVisibility interface {
	SharedRecipientAccountIDs(context.Context, int64, string, int64) ([]int64, error)
}

// RequestSharePolicy belongs to the owner of both append-only ledgers. A
// protection event permanently voids older choices, even after release.
type RequestSharePolicy struct{}

func (RequestSharePolicy) EffectiveRecipientSet(createdAt time.Time, accountIDs []int64, protected bool, protectedAt time.Time) map[int64]bool {
	selected := map[int64]bool{}
	if protected || (!protectedAt.IsZero() && !createdAt.After(protectedAt)) {
		return selected
	}
	for _, id := range accountIDs {
		selected[id] = true
	}
	return selected
}
