// ShiftPlanSyncer is the #1843 bridge between a sick report
// (active.staff_absences) and the planning layer (schedule.staff_shifts +
// schedule.instance_staff). The interface lives here because services/schedule
// imports services/active — the implementation (services/schedule) is injected
// via SetShiftPlanSyncer at factory wiring time, mirroring the AttendanceSyncer
// shape. Unlike that syncer the contract is FAIL-CLOSED: the linkage is the
// feature, so an error must abort the surrounding absence write (a sick report
// whose plan effects half-applied is exactly the silent side effect #1843
// forbids).
package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// SickCascadeInput identifies one sick report and the range it covers. Both
// cascade directions take the same shape.
type SickCascadeInput struct {
	SubjectStaffID int64
	DateStart      timezone.Date
	DateEnd        timezone.Date
	// SkipStartDay/SkipEndDay mirror start_half_day/end_half_day: a half sick
	// day never cascades (the person works the other half; which shift or
	// block that touches is the admin's call). The absence note is
	// deliberately NOT part of this input — plan surfaces are team-visible
	// and must never leak medical detail; the cascade writes neutral labels.
	SkipStartDay bool
	SkipEndDay   bool
	AbsenceID    int64
	// ActorStaffID stamps updated_by on cancelled shifts; ActorAccountID goes
	// into the deviation events. Both describe who filed/deleted the report.
	ActorStaffID   int64
	ActorAccountID *int64
}

// ShiftPlanSyncer cascades a sick report into the plans and back out again.
// Both methods run inside the caller's tenant transaction.
type ShiftPlanSyncer interface {
	// MarkSickForRange cancels the subject's own shifts (ChangeReason
	// "Krankheit", provenance-stamped) and marks their care-block rows absent
	// for the range. Idempotent: already-cancelled shifts and already-absent
	// rows are left untouched.
	MarkSickForRange(ctx context.Context, in SickCascadeInput) error
	// ClearSickForRange reverses exactly what MarkSickForRange stamped for
	// this absence id. Shifts that meanwhile received replacements and blocks
	// that received substitutes are skipped (never destroy admin work); their
	// stamps are released so the deleted report stops owning them.
	ClearSickForRange(ctx context.Context, in SickCascadeInput) error
	// ReconcileSickRange applies only the calendar-day difference between an
	// existing full-day sick report and its edited range. Days that remain sick
	// are not cleared/re-applied, so their provenance and audit history stay
	// stable. Both inputs refer to the same absence and subject.
	ReconcileSickRange(ctx context.Context, before, after SickCascadeInput) error
	// ReassignSickStamps re-points every provenance stamp from one absence id
	// to another. Needed by the overlap-merge path: merging sick reports
	// deletes the secondary absence rows, and their stamps must transfer to
	// the surviving primary or the eventual reversal will miss those rows.
	ReassignSickStamps(ctx context.Context, fromAbsenceID, toAbsenceID int64) error
}

// noopShiftPlanSyncer keeps the absence service functional when no syncer is
// wired (unit tests construct the service bare).
type noopShiftPlanSyncer struct{}

func (noopShiftPlanSyncer) MarkSickForRange(context.Context, SickCascadeInput) error {
	return nil
}

func (noopShiftPlanSyncer) ClearSickForRange(context.Context, SickCascadeInput) error {
	return nil
}

func (noopShiftPlanSyncer) ReconcileSickRange(context.Context, SickCascadeInput, SickCascadeInput) error {
	return nil
}

func (noopShiftPlanSyncer) ReassignSickStamps(context.Context, int64, int64) error {
	return nil
}
