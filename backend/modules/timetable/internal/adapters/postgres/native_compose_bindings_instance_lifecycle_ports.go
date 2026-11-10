package postgres

import (
	"context"

	"github.com/uptrace/bun"
)

// SubstituteDayLock takes one transaction-scoped advisory lock on the caller's
// transaction, for compositions that serialize with the owner's staffing
// writes (the #1843 sick cascade). The owner's lockScheduleDays derives the
// day key and admits it in the unit of work's lock order (#6232).
func SubstituteDayLock(db *bun.DB) func(context.Context, string) error {
	return New(DatabaseRuntime(db)).AcquireTransactionLock
}
