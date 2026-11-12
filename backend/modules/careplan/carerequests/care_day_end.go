package carerequests

import (
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CareDayEnded reports whether the school's care day is over at now.
// careDayEnd is the school's operations.session_end_time as HH:MM; without
// one the care day lasts the whole day. The end minute itself counts as over.
//
// It is the one rule for "Betreuungsende": "Im Unterricht" ends with it
// (careplan.InClassUntilCareDayEnd, #5656), and guardians can no longer change
// today's pickup once it has passed (#5710).
func CareDayEnded(careDayEnd string, now time.Time) bool {
	end := strings.TrimSpace(careDayEnd)
	return end != "" && now.In(calendar.Berlin).Format(sameDayCutoffLayout) >= end
}
