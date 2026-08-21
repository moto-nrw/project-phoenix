package schedule

// Day-planning reason codes describe why a child is expected (or not) on a
// given day. They are part of the students API wire contract
// (StudentResponse.day_planning_reason); the api layer aliases these constants
// and must keep the string values byte-identical.
const (
	DayPlanningReasonSick             = "sick"
	DayPlanningReasonExcused          = "excused"
	DayPlanningReasonClassTrip        = "class_trip"
	DayPlanningReasonArrivalException = "arrival_exception"
	DayPlanningReasonPickupException  = "pickup_exception"
	DayPlanningReasonArrivalSchedule  = "arrival_schedule"
	DayPlanningReasonPickupSchedule   = "pickup_schedule"
	DayPlanningReasonTimetable        = "timetable"
	DayPlanningReasonUnplanned        = "unplanned_attendance"
	DayPlanningReasonNoPlan           = "no_plan"
)

// DayPlanningInputs carries the already-resolved facts the precedence rules
// decide from. HasActualAttendance and the status flags are computed by the
// caller; Arrival and Pickup are the effective per-day schedule entries.
type DayPlanningInputs struct {
	HasActualAttendance bool
	Sick                bool
	ClassTrip           bool
	Excused             bool
	Arrival             *EffectiveArrivalTime
	Pickup              *EffectivePickupTime
	HasTimetable        bool
}

// DayPlanningDecision is the outcome of the precedence rules. ExceptionNotes
// carries the arrival/pickup exception note for the no-time exception cases so
// the caller can render it; it is empty otherwise.
type DayPlanningDecision struct {
	ComesToday     bool
	Reason         string
	ExceptionNotes string
}

// ResolveDayPlanning resolves the plan first. Actual attendance changes an
// absent/no-plan decision to unplanned attendance, but it does not erase a
// valid arrival, pickup, or timetable plan.
//
// A timeless exception on either leg is the "Kommt heute nicht" marker and
// cancels the whole care day — it must not fall through to the other leg's
// regular schedule time or a timetable booking (the contract
// mergeCareExceptions documents for the parent portal, #1725). Staff often
// cancel only the leg they are looking at; the child is still not coming. A
// timed exception on the OTHER leg does not overrule the cancellation either:
// the parent portal resolves `pickup_absent || arrival_absent` to an absence
// before it looks at any time (frontend/src/components/parent/child-care.tsx),
// so letting a time win here would make the guardian tile and the staff-side
// counts disagree about the same child on the same day. The student search
// (#1939) and the timetable's care-day derivation (#1747) arrived at this same
// precedence independently; both paths must keep resolving through this
// function so they never disagree.
func ResolveDayPlanning(in DayPlanningInputs) DayPlanningDecision {
	planned := resolvePlannedDay(in)
	if in.HasActualAttendance && !planned.ComesToday {
		return DayPlanningDecision{ComesToday: true, Reason: DayPlanningReasonUnplanned}
	}
	return planned
}

func resolvePlannedDay(in DayPlanningInputs) DayPlanningDecision {
	switch {
	case in.Sick:
		return DayPlanningDecision{Reason: DayPlanningReasonSick}
	case in.ClassTrip:
		return DayPlanningDecision{Reason: DayPlanningReasonClassTrip}
	case in.Excused:
		return DayPlanningDecision{Reason: DayPlanningReasonExcused}
	}

	// A day-specific exception that clears the arrival or pickup ("heute nicht
	// da") is an absence signal and has to win even when the child also carries
	// an unrelated presence signal — a regular pickup time, a timetable
	// placement — for the same day.
	if in.Arrival != nil && in.Arrival.IsException && in.Arrival.ArrivalTime == nil {
		return DayPlanningDecision{Reason: DayPlanningReasonArrivalException, ExceptionNotes: in.Arrival.Notes}
	}
	if in.Pickup != nil && in.Pickup.IsException && in.Pickup.PickupTime == nil {
		return DayPlanningDecision{Reason: DayPlanningReasonPickupException, ExceptionNotes: in.Pickup.Notes}
	}

	if in.Arrival != nil && in.Arrival.ArrivalTime != nil {
		if in.Arrival.IsException {
			return DayPlanningDecision{ComesToday: true, Reason: DayPlanningReasonArrivalException}
		}
		return DayPlanningDecision{ComesToday: true, Reason: DayPlanningReasonArrivalSchedule}
	}
	if in.Pickup != nil && in.Pickup.PickupTime != nil {
		if in.Pickup.IsException {
			return DayPlanningDecision{ComesToday: true, Reason: DayPlanningReasonPickupException}
		}
		return DayPlanningDecision{ComesToday: true, Reason: DayPlanningReasonPickupSchedule}
	}
	if in.HasTimetable {
		return DayPlanningDecision{ComesToday: true, Reason: DayPlanningReasonTimetable}
	}
	return DayPlanningDecision{Reason: DayPlanningReasonNoPlan}
}
