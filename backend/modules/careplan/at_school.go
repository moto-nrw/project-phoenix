package careplan

import (
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// InClassUntilCareDayEnd reports whether a child checked out with "Geht zum
// Unterricht" still reads "Im Unterricht" (#5656). A child who goes home straight
// from class never scans back in, so the status ends with the school's care day
// and the child reads "Zuhause" again. The child's own pickup time does not end
// it: an early-care child can be planned to leave before class even starts.
// Without a configured end the status holds for the rest of the day.
func InClassUntilCareDayEnd(careDayEnd string, now time.Time) bool {
	return !carerequests.CareDayEnded(careDayEnd, now)
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
	return in.Now.In(calendar.Berlin).Format("15:04") < end
}
