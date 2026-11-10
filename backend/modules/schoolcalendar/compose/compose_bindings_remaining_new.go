package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/traegercalendar"
)

// NewTraegerClosingDays builds a Träger's closing days (#4702) in its Träger transaction (ADR 0105) over its schools; notices tells the parents of affected holiday bookings (#6434).
func NewTraegerClosingDays(schools traegercalendar.Schools, notices traegercalendar.HolidayNotices) traegercalendar.ClosingDays {
	return application.NewTraegerClosingDays(postgres.NewTraegerClosingDayStore(), schools, notices)
}

// NewHolidayClosureEvents composes the notices of ADR 0196 over the caller's school or Träger transaction.
func NewHolidayClosureEvents() *postgres.HolidayClosureEventStore {
	return postgres.NewHolidayClosureEventStore(postgres.NewCompositionDatabase(), nil)
}
