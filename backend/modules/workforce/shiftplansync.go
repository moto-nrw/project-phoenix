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

// SickCascadeStepKind names one plan change a sick-report write asks for.
type SickCascadeStepKind string

const (
	// SickCascadeMark cascades Report into the plans: its shifts are
	// cancelled and its care-block rows marked absent.
	SickCascadeMark SickCascadeStepKind = "mark"
	// SickCascadeClear reverses what Report stamped.
	SickCascadeClear SickCascadeStepKind = "clear"
	// SickCascadeReconcile applies the calendar-day difference between the
	// stored range Before and the edited range Report of one sick report.
	SickCascadeReconcile SickCascadeStepKind = "reconcile"
	// SickCascadeReassign re-points the stamps of the merged report
	// FromAbsenceID to the surviving report ToAbsenceID.
	SickCascadeReassign SickCascadeStepKind = "reassign"
)

// SickCascadeStep is one plan change of a sick-report write.
type SickCascadeStep struct {
	Kind SickCascadeStepKind
	// Report is the sick report of Mark and Clear and the edited report of
	// Reconcile.
	Report SickCascadeInput
	// Before is the stored range of Reconcile.
	Before SickCascadeInput
	// FromAbsenceID and ToAbsenceID are the merged and the surviving report
	// of Reassign.
	FromAbsenceID int64
	ToAbsenceID   int64
}

// SickCascadePlan is the ordered plan work of one absence write (#1843): the
// changes to the Dienstplan (this owner's schedule.staff_shifts) and the
// Betreuungsplan staffing (Timetable's schedule.instance_staff) the write
// asks for. Workforce applies none of them. The shift-plan-sync workflow runs
// the write and its plan in one tenant transaction, because the two writes
// cross an ownership line, and the plan is FAIL-CLOSED: a failed step aborts
// the absence write (#4825).
type SickCascadePlan []SickCascadeStep

// SickReportWrites are the absence writes that may cascade a sick report
// into the plans. Each runs in the caller's tenant transaction and returns
// the plan it asks for beside its result; only the shift-plan-sync workflow
// calls them, and it applies the plan before the transaction commits.
type SickReportWrites interface {
	CreateOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, request CreateAbsenceRequest) (*StaffAbsenceResponse, SickCascadePlan, error)
	CreateAbsenceFor(ctx context.Context, subjectStaffID, createdByStaffID int64, actorAccountID *int64, request CreateAbsenceRequest) (*StaffAbsenceResponse, SickCascadePlan, error)
	UpdateAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, request UpdateAbsenceRequest) (*StaffAbsenceResponse, SickCascadePlan, error)
	DeleteOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64) (SickCascadePlan, error)
	DeleteAbsenceFor(ctx context.Context, subjectStaffID, actorStaffID int64, actorAccountID *int64, absenceID int64) (SickCascadePlan, error)
}
