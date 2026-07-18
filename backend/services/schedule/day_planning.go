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

// ResolveDayPlanning applies the day-planning precedence: actual attendance
// beats a reported absence (sick, class trip, excused), which beats a planned
// arrival, which beats a planned pickup, which beats a timetable booking. A
// no-time arrival/pickup exception marks the child as absent, and everything
// else falls through to "no plan".
func ResolveDayPlanning(in DayPlanningInputs) DayPlanningDecision {
	switch {
	case in.HasActualAttendance:
		return DayPlanningDecision{ComesToday: true, Reason: DayPlanningReasonUnplanned}
	case in.Sick:
		return DayPlanningDecision{Reason: DayPlanningReasonSick}
	case in.ClassTrip:
		return DayPlanningDecision{Reason: DayPlanningReasonClassTrip}
	case in.Excused:
		return DayPlanningDecision{Reason: DayPlanningReasonExcused}
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
	if in.Arrival != nil && in.Arrival.IsException && in.Arrival.ArrivalTime == nil {
		return DayPlanningDecision{Reason: DayPlanningReasonArrivalException, ExceptionNotes: in.Arrival.Notes}
	}
	if in.Pickup != nil && in.Pickup.IsException && in.Pickup.PickupTime == nil {
		return DayPlanningDecision{Reason: DayPlanningReasonPickupException, ExceptionNotes: in.Pickup.Notes}
	}
	return DayPlanningDecision{Reason: DayPlanningReasonNoPlan}
}
