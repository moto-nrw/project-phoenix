package careplan

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CareDayQuery resolves the care plan without treating attendance or a
// timetable assignment as a booking. Range reads batch their source facts.
type CareDayQuery interface {
	ResolveForDate(context.Context, []int64, calendar.Date) (map[int64]CareDayStatus, error)
	ResolveForRange(context.Context, []int64, calendar.Date, calendar.Date) (map[int64]map[calendar.Date]CareDayStatus, error)
}

// CareDayStatus is the derived per-child, per-day care-plan verdict.
type CareDayStatus string

const (
	// CareDayScheduled — the care plan puts the child in the OGS that day.
	CareDayScheduled CareDayStatus = "scheduled"

	// CareDayNotScheduled — the child is not booked into care that day: no
	// arrival and no pickup plans the weekday, and nobody said otherwise. Not
	// expected, and never an absence — there was no care to miss.
	CareDayNotScheduled CareDayStatus = "not_scheduled"

	// CareDayCancelled — somebody explicitly cancelled the day for this child
	// ("Kommt heute nicht": a same-day arrival or pickup exception without a
	// time). Not expected either, but this is a REPORTED ABSENCE, not a
	// non-booking: it must still be stamped absent when the block ends so the
	// attendance history and the exports keep showing it (#1747 review).
	CareDayCancelled CareDayStatus = "cancelled"

	// CareDayUnknown — no care plan on file at all, so the plan cannot say
	// anything about that day. Treated as expected: schools that do not
	// maintain arrival/pickup plans must keep seeing their full roster, and a
	// child must never disappear because data is missing.
	CareDayUnknown CareDayStatus = "unknown"
)

// Expected reports whether a child with this status belongs in the expected
// count. Both "not booked" and "cancelled" say the child is not coming; only a
// missing or affirmative plan keeps them in.
func (s CareDayStatus) Expected() bool {
	return s != CareDayNotScheduled && s != CareDayCancelled
}

// ExemptFromAbsence reports whether ending a block may skip the
// expected → absent stamp for this child.
//
// ONLY a non-booking qualifies. A cancellation looks the same on the planner
// (not expected), but writing no row would erase the absence from the
// attendance history and the exports — the day WAS booked and the child did
// not come, which is exactly what an absence records.
func (s CareDayStatus) ExemptFromAbsence() bool { return s == CareDayNotScheduled }

// CareDayFacts contains the resolved plan facts for one child and day.
// Attendance and status-day absences belong to the roster, not this query.
type CareDayFacts struct {
	BookingsAuthoritative bool
	HasBookedCareDay      bool
	HasPlan               bool
	HasArrivalSchedule    bool
	Arrival               *EffectiveArrivalTime
	Pickup                *EffectivePickupTime
}

// ResolveCareDay derives the plan verdict without loading attendance or
// treating a timetable assignment as a care booking.
func ResolveCareDay(facts CareDayFacts) CareDayStatus {
	if facts.BookingsAuthoritative && !facts.HasBookedCareDay {
		return CareDayNotScheduled
	}
	decision := ResolveDayPlanning(DayPlanningInputs{Arrival: facts.Arrival, Pickup: facts.Pickup})
	switch {
	case decision.ComesToday:
		return CareDayScheduled
	case decision.Reason == DayPlanningReasonArrivalException || decision.Reason == DayPlanningReasonPickupException:
		return CareDayCancelled
	case decision.Reason == DayPlanningReasonNoPlan && !facts.HasPlan:
		return CareDayUnknown
	case facts.HasPlan && facts.HasArrivalSchedule:
		return CareDayScheduled
	default:
		return CareDayNotScheduled
	}
}
