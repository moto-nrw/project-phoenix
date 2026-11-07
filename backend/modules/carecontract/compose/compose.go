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

// NewContracts builds the contracts of the Träger-Büro. The writes run in
// the Träger transaction of the portal route; the reads open their own units
// for the access log of each school.
func NewContracts(ports ContractPorts) carecontract.Contracts {
	return application.NewContracts(postgres.NewStore(), rendering.NewRenderer(), ports, application.Clock{Now: time.Now})
}

// ContractPorts are the ports the root binds for the contracts.
type ContractPorts = application.ContractPorts

// NewIntake builds the creation of contracts from the accepted applications
// and the templates of the offerings; it opens its own units.
func NewIntake(ports IntakePorts) carecontract.Intake {
	return application.NewIntake(postgres.NewStore(), ports, calendar.TodayDate)
}

// IntakePorts are the ports the root binds for the intake.
type IntakePorts = application.IntakePorts

// NewConclusion builds the conclusion at the acceptance (Weg b, #4680)
// over the ports of the contracts and of the intake; it opens its own units.
func NewConclusion(contracts ContractPorts, intake IntakePorts) carecontract.Conclusion {
	clock := application.Clock{Now: time.Now}
	return application.NewConclusions(application.NewIntake(postgres.NewStore(), intake, calendar.TodayDate),
		application.NewContracts(postgres.NewStore(), rendering.NewRenderer(), contracts, clock), clock)
}

// NewInbox builds the contracts of the parents' inbox, in the Träger
// transaction the parents portal opens.
func NewInbox() carecontract.Inbox { return application.NewInbox(postgres.NewStore()) }

// NewContractMailRenderer renders the mail of a contract for Delivery's
// worker.
func NewContractMailRenderer() carecontract.ContractMailRenderer {
	return application.RenderContractMail
}
