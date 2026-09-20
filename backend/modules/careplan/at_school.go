package careplan

import (
	"strings"
	"time"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// AtSchoolInputs carries the facts that decide whether a child who is not
// checked in yet is still in class rather than at home (#3260).
//
// Decision is today's resolved day plan. CheckedInToday is true once today's
// attendance has any record, including a checkout. PickupTime is today's
// effective pickup time (nil when the plan has none); CareDayEnd is the
// school's "HH:MM" session end, the fallback when there is no pickup time.
type AtSchoolInputs struct {
	Decision       DayPlanningDecision
	CheckedInToday bool
	PickupTime     *time.Time
	CareDayEnd     string
	Now            time.Time
}

// IsAtSchoolBeforeCheckIn reports whether a child should read "Schule"
// instead of "Zuhause": the plan expects the child today, nothing has been
// recorded yet, and the child's care day is not over.
//
// Sick, excused, class trip and "Kommt heute nicht" never qualify because
// ResolveDayPlanning already resolves them to not coming. After the first
// check-in the live location takes over, and after a checkout the child is
// at home again, so any attendance record ends the state. The end of the care
// day is the child's own pickup time; without one it is the school's session
// end. Deliberately no per-school "school ends" time: the decision in #3260
// was "until the first check-in", not a clock rule.
func IsAtSchoolBeforeCheckIn(in AtSchoolInputs) bool {
	if !in.Decision.ComesToday || in.Decision.Reason == DayPlanningReasonUnplanned {
		return false
	}
	if in.CheckedInToday {
		return false
	}
	end := strings.TrimSpace(in.CareDayEnd)
	if in.PickupTime != nil {
		end = in.PickupTime.Format("15:04")
	}
	if end == "" {
		return true
	}
	return in.Now.In(timezone.Berlin).Format("15:04") < end
}
