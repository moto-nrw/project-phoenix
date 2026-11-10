package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/traegercalendar"
)

// NewTraegerClosingDays builds a Träger's closing days (#4702) in its Träger transaction (ADR 0105) over its schools; notices tells the parents of affected holiday bookings (#6434).
func NewTraegerClosingDays(schools traegercalendar.Schools, notices traegercalendar.HolidayNotices) traegercalendar.ClosingDays {
	return application.NewTraegerClosingDays(postgres.NewTraegerClosingDayStore(), schools, notices)
}

// HolidayClosureEvents is the proposal and worker side of the notices after
// a school closing day changed (ADR 0196).
type HolidayClosureEvents interface {
	schoolcalendar.HolidayClosureProposals
	schoolcalendar.HolidayClosureEventQueue
}

// NewHolidayClosureEvents composes the proposals and commands over the
// caller's school or Träger transaction.
func NewHolidayClosureEvents() HolidayClosureEvents {
	return application.NewHolidayClosureEvents(postgres.NewHolidayClosureEventStore(postgres.NewCompositionDatabase()), nil)
}
