package parentbilling

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// HolidayWeekNoCareDayRefusal preserves the calendar reason a rebooking's
// registrations port reports, without depending on Enrollment's types.
type HolidayWeekNoCareDayRefusal struct {
	Week   int
	WeekID int64
	Date   calendar.Date
	Kind   string
	Name   string
}

func (r *HolidayWeekNoCareDayRefusal) Error() string {
	return fmt.Sprintf("week %d: %s is a day without care (%s: %s)", r.Week+1, r.Date, r.Kind, r.Name)
}
