package timetable

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Roster maintenance keeps already-materialized occurrences in line with
// roster changes the insert-only materializer never revisits (#405, #2147):
// a grade transition graduating or restoring children, and the enrollment
// resync of an offering-sourced template. Every method runs in the caller's
// tenant transaction.

// RosterEnrollment is one enrollment row as the roster decision reads it:
// the validity window, the calendar period and weekday scope, and whether the
// child has graduated.
type RosterEnrollment struct {
	StudentID        int64
	ValidFrom        calendar.Date
	ValidUntil       *calendar.Date
	CalendarPeriodID *int64
	Weekday          *int
	SelectedWeekdays []int
	StudentAlumnus   bool
}

// RosterMaintenance reconciles materialized rosters.
type RosterMaintenance interface {
	// RemoveStudentsFromFutureRosters deletes the still-planned attendance
	// rows of graduated children from today on, archiving every removed row
	// under transitionID.
	RemoveStudentsFromFutureRosters(ctx context.Context, transitionID int64, studentIDs []int64) error
	// CurrentRosterBaseline is the ordering marker an apply records so its
	// revert can tell instances materialized during the alumnus window.
	CurrentRosterBaseline(ctx context.Context) (int64, error)
	// RestoreStudentsToFutureRosters undoes the removal for reactivated
	// children; a nil baseline skips the enrollment refill.
	RestoreStudentsToFutureRosters(ctx context.Context, transitionID int64, studentIDs []int64, baselineInstanceID *int64) error
	// ReconcileSourcedTemplateRosters aligns one template's future planned
	// occurrences with its current enrollments for the given children.
	// prior is the enrollment state before the caller's writes; nil
	// re-establishes coverage from scratch. Returns rows created and removed.
	ReconcileSourcedTemplateRosters(ctx context.Context, templateID int64, studentIDs []int64, from calendar.Date, prior []RosterEnrollment) (int, int, error)
}

// RecurrenceWriteLock is the tenant-wide gate that orders every write of
// recurrence-derived state across modules: template edits, splits, re-plans,
// materialization, care-offering rosters and the other recurrence writers.
// The locks are transaction-scoped advisory locks; the caller must already be
// inside its tenant transaction so the lock covers the complete mutation.
// Lock order is fixed: the recurrence gate first, School Structure's grade
// transition gate second.
type RecurrenceWriteLock interface {
	// LockRecurrenceWrites takes the tenant recurrence gate.
	LockRecurrenceWrites(ctx context.Context) error
	// LockRecurrenceWritesThenGradeTransitions takes the recurrence gate and
	// then the grade transition gate, for writers that decide roster rows
	// from the children's lifecycle status.
	LockRecurrenceWritesThenGradeTransitions(ctx context.Context) error
}
