package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/traegercalendar"
	"github.com/uptrace/bun"
)

func retainedNewService(db *bun.DB, observe func(ports.Observation), administration application.Administration) *application.Service {
	store := postgres.New(postgres.NewCompositionDatabase())
	return application.New(store, ports.Observer(observe), administration, application.LocalCalendar{})
}

// NewTraegerClosingDays builds a Träger's closing days (#4702) in its Träger transaction (ADR 0105) over its schools.
func NewTraegerClosingDays(schools traegercalendar.Schools) traegercalendar.ClosingDays {
	return application.NewTraegerClosingDays(postgres.NewTraegerClosingDayStore(), schools)
}
