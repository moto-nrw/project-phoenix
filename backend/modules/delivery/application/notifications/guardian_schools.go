package notifications

import "context"

// GuardianSchools supplies eligible schools for an authenticated guardian.
// Implementations join the caller's administrative transaction.
type GuardianSchools interface {
	ListGuardianSchoolIDs(context.Context, int64) ([]int64, error)
}
