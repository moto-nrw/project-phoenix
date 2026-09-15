// Package legacy keeps the retained staff absence and group substitution
// repository contracts (models/active, models/education) alive on top of the
// Workforce capability while their consumers migrate (#2688). It performs no
// persistence of its own: every adapter maps the legacy models onto the public
// capability types and preserves the error shapes those callers still classify
// on. The composition root constructs the adapters; nothing else imports them.
package legacy

import (
	"fmt"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

// rowsAffectedError is the legacy repository result for an update that
// matched no row: the same DatabaseError the generic repository produced.
func rowsAffectedError(op string) error {
	return &modelBase.DatabaseError{Op: op, Err: fmt.Errorf("expected %d rows affected, got %d", 1, 0)}
}

// uniqueIDs drops non-positive and repeated IDs, keeping first-seen order.
func uniqueIDs(ids []int64) []int64 {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, found := seen[id]; found {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
