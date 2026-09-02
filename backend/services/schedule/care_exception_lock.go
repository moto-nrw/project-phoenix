package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/uptrace/bun"
)

// BindCareStudentLockForDB installs a graph-scoped owner lock for the
// database used by the timetable services.
func BindCareStudentLockForDB(db *bun.DB, lock func(context.Context, int64) error, notFound error) {
	careplanning.BindStudentLockForDB(db, lock, notFound)
}

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

// LockCareStudent takes only the student row FOR UPDATE — the shared first
// lock of every care-day writer. Weekly-schedule writers acquire it before
// touching schedule rows so their later per-day locks (auto-excusal resync)
// keep the student → day order instead of inverting it.
func LockCareStudent(ctx context.Context, db *bun.DB, studentID int64) error {
	return careplanning.LockStudent(ctx, db, studentID)
}
