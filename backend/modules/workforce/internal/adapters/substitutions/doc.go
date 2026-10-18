// Package legacy keeps the retained group substitution repository contract
// alive on top of the Workforce capability while its consumers migrate
// (#2688). It performs no persistence of its own: the adapter maps the
// retained rows onto the public capability types and preserves the error
// shapes those callers still classify on. The composition root constructs the
// adapter; nothing else imports it. The staff absence adapters moved into
// modules/workforce/compose (#3996).
package legacy

import "fmt"

// databaseError is the retained repository error shape the callers still
// see: the failed operation in front of its cause, the cause in the chain,
// and the store-failure mark. It reads exactly as the generic repository's
// DatabaseError did.
type databaseError struct {
	op  string
	err error
}

func (e *databaseError) Error() string {
	if e.err == nil {
		return "database error during " + e.op
	}
	return "database error during " + e.op + ": " + e.err.Error()
}

func (e *databaseError) Unwrap() error { return e.err }

// StoreFailure marks the error as the store's failure rather than a refusal
// of the request.
func (e *databaseError) StoreFailure() bool { return true }

// errRecordNotFound is the persistence-neutral missing-row result. It carries
// the repository not-found marker the callers classify a missing row by.
var errRecordNotFound error = recordNotFoundError{}

type recordNotFoundError struct{}

func (recordNotFoundError) Error() string { return "repository: not found" }

// RepositoryNotFound marks the not-found result by shape.
func (recordNotFoundError) RepositoryNotFound() {}

// rowsAffectedError is the legacy repository result for an update that
// matched no row, as the generic repository produced it.
func rowsAffectedError(op string) error {
	return &databaseError{op: op, err: fmt.Errorf("expected %d rows affected, got %d", 1, 0)}
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
