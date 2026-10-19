package application

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Clock derives the date and instant from the same injected clock, so a
// pinned date never disagrees with the live work-session window at midnight.
type Clock struct{ Time func() time.Time }

func (c Clock) Now() time.Time { return c.Time() }
func (c Clock) Today() string  { return calendar.DateFromTime(c.Time()).String() }
