package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/traegercalendar"
)

// NewTraegerClosingDays builds the closing days of a Träger (#4702) in the
// Träger-Portal's Träger transaction (ADR 0105); schools names the Träger's
// schools with their federal state.
func NewTraegerClosingDays(schools traegercalendar.Schools) traegercalendar.ClosingDays {
	return application.NewTraegerClosingDays(postgres.NewTraegerClosingDayStore(), schools)
}
