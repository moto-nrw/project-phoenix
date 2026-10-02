package application

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
)

func normalizeWallClock(value time.Time) time.Time {
	return time.Date(1, time.January, 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}

func todayDate() appointments.Date {
	// The existing appointment date owns Berlin calendar-day semantics.
	location := appointments.NewDate(2000, 1, 1).BerlinMidnight().Location()
	value := time.Now().In(location)
	return appointments.NewDate(value.Year(), value.Month(), value.Day())
}

// today is the feed's calendar day: the injected one, else the real one.
func (s *service) today() appointments.Date {
	if s.cfg.Today != nil {
		return s.cfg.Today()
	}
	return todayDate()
}
