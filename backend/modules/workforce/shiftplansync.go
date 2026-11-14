package workforce

import "context"

// SickShiftChangeReason is the neutral change reason of a shift a sick
// report cancels (#1843): the sick cascade writes it on shifts planned before
// the report, the series materializer on shifts planned after it (#6302). The
// absence note never reaches the plan: the Dienstplan is team-visible and the
// note may carry private medical detail.
const SickShiftChangeReason = "Krankheit"

// AbsenceShiftChangeReason is the neutral change reason of a shift every
// other absence cancels (#6847): Urlaub, Fortbildung, Freizeitausgleich,
// Sonstige and the school's own Abwesenheitsarten. The plan never names the
// art, because a school-defined name may be private.
const AbsenceShiftChangeReason = "Abwesenheit"

// ShiftChangeReasonFor is the change reason an absence of the canonical type
// writes on the shifts it cancels.
func ShiftChangeReasonFor(absenceType string) string {
	if absenceType == AbsenceTypeSick {
		return SickShiftChangeReason
	}
	return AbsenceShiftChangeReason
}

// IsAbsenceShiftChangeReason reports whether reason is a label the plan
// cascade writes, which a reactivated shift drops again.
func IsAbsenceShiftChangeReason(reason string) bool {
	return reason == SickShiftChangeReason || reason == AbsenceShiftChangeReason
}

// PlanEffectiveAbsenceStatuses are the statuses with which an absence takes
// plan effect: a reported entry and an approved request (#6847). A pending,
// questioned, declined or canceled request leaves the plans alone.
func PlanEffectiveAbsenceStatuses() []string {
	return []string{AbsenceStatusReported, AbsenceStatusApproved}
}

// SickCascadeInput identifies one absence and the range it covers (#1843).
// Since #6847 every absence with plan effect cascades, not only a sick
// report; AbsenceType decides the labels. Both cascade directions take the
// same shape. DateStart and DateEnd are calendar days in DateLayout.
type SickCascadeInput struct {
	SubjectStaffID int64
	AbsenceType    string
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
//
// Since #6847 these are all absence writes with plan effect: an approval
// cascades the approved request, a withdrawal reverses it.
type SickReportWrites interface {
	ApproveAbsence(ctx context.Context, absenceID, actorAccountID, decidedByStaffID int64, note string) (*StaffAbsenceResponse, SickCascadePlan, error)
	CancelAbsence(ctx context.Context, staffID, actorAccountID, absenceID int64) (SickCascadePlan, error)
	CreateOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, request CreateAbsenceRequest) (*StaffAbsenceResponse, SickCascadePlan, error)
	CreateAbsenceFor(ctx context.Context, subjectStaffID, createdByStaffID int64, actorAccountID *int64, request CreateAbsenceRequest) (*StaffAbsenceResponse, SickCascadePlan, error)
	UpdateAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, request UpdateAbsenceRequest) (*StaffAbsenceResponse, SickCascadePlan, error)
	DeleteOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64) (SickCascadePlan, error)
	DeleteAbsenceFor(ctx context.Context, subjectStaffID, actorStaffID int64, actorAccountID *int64, absenceID int64) (SickCascadePlan, error)
}

// AbsencePlanGap is one absence in force whose plan rows from today on do
// not yet carry its effect (#7021): shifts still planned on its full days
// and care-block rows still staffed. An absence entered before every type
// cascaded (#6847) left the rows planned at that time untouched; the cascade
// re-applied over it closes the gap without overriding hand planning.
// DateStart and DateEnd are calendar days in DateLayout.
type AbsencePlanGap struct {
	AbsenceID   int64  `json:"absence_id"`
	StaffID     int64  `json:"staff_id"`
	AbsenceType string `json:"absence_type"`
	DateStart   string `json:"date_start"`
	DateEnd     string `json:"date_end"`
	OpenShifts  int    `json:"open_shifts"`
	OpenBlocks  int    `json:"open_blocks"`
}
