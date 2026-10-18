package compose

import (
	usersRepo "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/peoplepostgres"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ParseRetainedCalendarDate parses a YYYY-MM-DD value the way the retained
// repositories do: an empty or malformed value is no date.
func ParseRetainedCalendarDate(value string) *calendar.Date {
	return usersRepo.ParseCalendarDate(value)
}
