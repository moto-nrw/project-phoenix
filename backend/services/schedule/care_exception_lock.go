package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/uptrace/bun"
)

// LockCareExceptionDay serializes pickup and arrival exception writes for one
// child-day. The parent portal treats staff ownership as day-level state, while
// the data lives in two tables, so every writer must take the same lock before
// checking or mutating either leg.
//
// Order is student row FOR UPDATE, then the care-day advisory lock — matching
// full-day status writers and partial-absence writes so concurrent paths cannot
// deadlock on the student FK vs care-day pair.
func LockCareExceptionDay(ctx context.Context, db *bun.DB, studentID int64, date timezone.Date) error {
	return careplanning.LockStudentAndExceptionDay(ctx, db, studentID, date)
}
