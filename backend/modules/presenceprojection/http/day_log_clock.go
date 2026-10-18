package presenceprojectionhttp

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// dayLogClock freezes one request's notion of "now". Date validation and every
// live-day decision must read the SAME instant: a request that starts at
// 23:59:59 Berlin and reaches the care-plan branch after midnight would
// otherwise validate against yesterday and then treat that date as historical,
// skipping care-plan resolution and reporting scheduled children as absent.
type dayLogClock struct {
	now   time.Time
	today calendar.Date
}

func (c dayLogClock) isToday(date calendar.Date) bool { return date == c.today }
