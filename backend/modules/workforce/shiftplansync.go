package workforce

import "context"

// SickCascadeInput identifies one sick report and the range it covers
// (#1843). Both cascade directions take the same shape. DateStart and
// DateEnd are calendar days in DateLayout.
type SickCascadeInput struct {
	SubjectStaffID int64
	DateStart      string
	DateEnd        string
	// SkipStartDay/SkipEndDay mirror start_half_day/end_half_day: a half sick
	// day never cascades (the person works the other half; which shift or
	// block that touches is the admin's call). The absence note is
	// deliberately NOT part of this input: plan surfaces are team-visible
	// and must never leak medical detail; the cascade writes neutral labels.
	SkipStartDay bool
	SkipEndDay   bool
	AbsenceID    int64
	// ActorStaffID stamps updated_by on cancelled shifts; ActorAccountID goes
	// into the deviation events. Both describe who filed/deleted the report.
	ActorStaffID   int64
	ActorAccountID *int64
}

// ShiftPlanSync is the port the absence lifecycle cascades a sick report
// through (#1843): into the Dienstplan (this owner's schedule.staff_shifts)
// and the Betreuungsplan staffing (Timetable's schedule.instance_staff) and
// back out again. Workforce declares the port; the shift-plan-sync
// application workflow implements it, because the two writes cross an
// ownership line inside one tenant transaction. The contract is FAIL-CLOSED:
// the linkage is the feature, so an error must abort the surrounding
// absence write. Every method runs inside the caller's tenant transaction.
type ShiftPlanSync interface {
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
	// existing full-day sick report and its edited range. Days that remain
	// sick are not cleared and re-applied, so their provenance and audit
	// history stay stable. Both inputs refer to the same absence and subject.
	ReconcileSickRange(ctx context.Context, before, after SickCascadeInput) error
	// ReassignSickStamps re-points every provenance stamp from one absence id
	// to another. Needed by the overlap-merge path: merging sick reports
	// deletes the secondary absence rows, and their stamps must transfer to
	// the surviving primary or the eventual reversal will miss those rows.
	ReassignSickStamps(ctx context.Context, fromAbsenceID, toAbsenceID int64) error
}
