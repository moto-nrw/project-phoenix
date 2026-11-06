// Package compose builds the Betreuungsvertrag (care-contract, ADR 0088,
// ADR 0153) over the ports the root binds.
package compose

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/adapters/rendering"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/application"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewContracts builds the contracts of the Träger-Büro, which run in the
// Träger transaction of the portal route.
func NewContracts(letterheads carecontract.Letterheads, post carecontract.Post) carecontract.Contracts {
	return application.NewContracts(postgres.NewStore(), letterheads, rendering.NewRenderer(), post, application.Clock{Now: time.Now})
}

// NewIntake builds the creation of contracts from the accepted applications
// and the templates of the offerings; it opens its own units.
func NewIntake(ports IntakePorts) carecontract.Intake {
	return application.NewIntake(postgres.NewStore(), application.IntakePorts(ports), calendar.TodayDate)
}

// IntakePorts are the ports the root binds for the intake.
type IntakePorts struct {
	Schools      carecontract.TraegerSchools
	Applications carecontract.SchoolApplications
	Bookings     carecontract.SchoolBookings
	Access       carecontract.SchoolAccessLog
	Runtime      carecontract.Runtime
}

// NewInbox builds the contracts of the parents' inbox, in the Träger
// transaction the parents portal opens.
func NewInbox() carecontract.Inbox { return application.NewInbox(postgres.NewStore()) }

// NewContractMailRenderer renders the mail of a contract for Delivery's
// worker.
func NewContractMailRenderer() carecontract.ContractMailRenderer {
	return application.RenderContractMail
}
