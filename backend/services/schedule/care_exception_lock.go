package schedule

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	if tenant.FromContext(ctx) <= 0 {
		return errors.New("tenant id is required")
	}
	err := careplanning.LockStudentAndExceptionDay(ctx, db, studentID, date.String())
	if errors.Is(err, careplanning.ErrStudentNotFound) {
		return sql.ErrNoRows
	}
	return err
}

// LockCareStudent takes only the student row FOR UPDATE — the shared first
// lock of every care-day writer. Weekly-schedule writers acquire it before
// touching schedule rows so their later per-day locks (auto-excusal resync)
// keep the student → day order instead of inverting it.
func LockCareStudent(ctx context.Context, db *bun.DB, studentID int64) error {
	err := careplanning.LockStudent(ctx, db, studentID)
	if errors.Is(err, careplanning.ErrStudentNotFound) {
		return sql.ErrNoRows
	}
	return err
}
