package timetable

import (
	"context"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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

type SourcedTemplateRosterReconcile struct {
	TemplateID int64
	StudentIDs []int64
	From       calendar.Date
	Prior      []RosterEnrollment
}

// RosterMaintenance reconciles materialized rosters.
type RosterMaintenance interface {
	ReconcileSourcedTemplateRosterBatch(context.Context, []SourcedTemplateRosterReconcile) error
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

// CohortChange is one child's Klasse or Gruppe write (#6841): what changed
// and the value before the write. The current values are read from the
// directory, so the caller names only the previous ones.
type CohortChange struct {
	StudentID           int64
	SchoolClassChanged  bool
	PreviousSchoolClass string
	GroupChanged        bool
	PreviousGroupID     *int64
}

// CohortRosters lets the already-materialized occurrences of Regeltermine
// with a Klasse, Jahrgang or Gruppe target follow children who change their
// Klasse or Gruppe (#6841). The materializer resolves those targets only when
// it creates an occurrence, and a whole planning period is materialized when
// a Regeltermin is saved, so without this pass a child who moved from 3a to
// 3b stays on every future 3a occurrence and is missing on the 3b ones.
type CohortRosters interface {
	// FollowCohortChanges adds the children to the planned occurrences from
	// `from` on (the caller passes today) of every template whose target they
	// joined, and removes their still-planned rows from the templates whose
	// target they left, unless an enrollment of that template still plans
	// them. Templates the change does not move the child into or out of stay
	// untouched, so hand edits there survive. The caller holds the tenant
	// recurrence gate and runs inside its tenant transaction.
	FollowCohortChanges(ctx context.Context, changes []CohortChange, from calendar.Date) error
}
