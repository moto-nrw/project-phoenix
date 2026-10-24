// Package shiftplansync is the application workflow for the two writes that
// cross Workforce and Timetable & Activities inside one tenant transaction:
// the #1843 sick cascade, which fans a sick report out into shift
// cancellations (Dienstplan, schedule.staff_shifts) and per-day block
// absences (Betreuungsplan, schedule.instance_staff) and reverses exactly the
// rows it stamped, and the schedule substitution moves, which pull a person
// from their blocks and put a stand-in there.
//
// Neither write has a single owner: the sick cascade follows Workforce's
// absence writes but touches Timetable's staffing rows, and a substitution is
// planned against Timetable's blocks but reads Workforce's people. Both are
// therefore entries of this workflow, behind the owners' public contracts:
// the root routes the absence writes and the schedule substitution target
// here, and no owner calls back into the workflow (#4825). The sick cascade
// is FAIL-CLOSED: the linkage is the feature, so an error must abort the
// surrounding absence write.
package shiftplansync

import (
	"context"

	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// ScheduleSubstitution is the Terminvertretung entry of /api/substitutions in
// School Structure's substitution vocabulary: the root routes the schedule
// target to it (#4825).
type ScheduleSubstitution interface {
	Overview(ctx context.Context, caller education.Caller, query education.OverviewQuery) (*education.OverviewResult, error)
	Assign(ctx context.Context, caller education.Caller, assignment education.Assignment) (*education.AssignmentResult, error)
	End(ctx context.Context, caller education.Caller, request education.EndRequest) error
}

// SickCascade is the #1843 cascade: it fans a sick report out into the
// Dienstplan and the Betreuungsplan and reverses exactly what it stamped.
// Every method runs inside the caller's tenant transaction.
type SickCascade interface {
	// MarkSickForRange cancels the subject's own shifts (ChangeReason
	// "Krankheit", provenance-stamped) and marks their care-block rows absent
	// for the range. Idempotent: already-cancelled shifts and already-absent
	// rows are left untouched.
	MarkSickForRange(ctx context.Context, in workforce.SickCascadeInput) error
	// ClearSickForRange reverses exactly what MarkSickForRange stamped for
	// this absence id. Shifts that meanwhile received replacements and blocks
	// that received substitutes are skipped (never destroy admin work); their
	// stamps are released so the deleted report stops owning them.
	ClearSickForRange(ctx context.Context, in workforce.SickCascadeInput) error
	// ReconcileSickRange applies only the calendar-day difference between an
	// existing full-day sick report and its edited range. Days that remain
	// sick are not cleared and re-applied, so their provenance and audit
	// history stay stable. Both inputs refer to the same absence and subject.
	ReconcileSickRange(ctx context.Context, before, after workforce.SickCascadeInput) error
	// ReassignSickStamps re-points every provenance stamp from one absence id
	// to another. Merging sick reports deletes the secondary absence rows,
	// and their stamps must transfer to the surviving primary or the
	// eventual reversal will miss those rows.
	ReassignSickStamps(ctx context.Context, fromAbsenceID, toAbsenceID int64) error
}
