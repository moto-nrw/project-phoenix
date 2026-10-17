package ports

import (
	"context"
)

// GuardianEnrollmentBackfill links the enrollment requests a guardian filed
// before they had an account to that account.
type GuardianEnrollmentBackfill interface {
	BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error)
}
