// Package compose builds the Betreuungsvertrag (ADR 0088, ADR 0153) over the ports the root binds.
package compose

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/adapters/rendering"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/application"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ContractPorts and IntakePorts are the ports the root binds.
type (
	ContractPorts = application.ContractPorts
	IntakePorts   = application.IntakePorts
)

// NewContracts builds the contracts of the Träger-Büro; the reads open own units for each school's access log.
func NewContracts(ports ContractPorts) carecontract.Contracts {
	return application.NewContracts(postgres.NewStore(), rendering.NewRenderer(), ports, application.Clock{Now: time.Now}, NewContractVersions())
}

// NewContractVersions reads the versions in force for the billing (#4681, #4684) in the caller's Träger unit.
func NewContractVersions() carecontract.ContractVersions {
	return application.NewVersions(postgres.NewStore())
}

// NewIntake builds the creation of contracts from the accepted applications and the offerings' templates.
func NewIntake(ports IntakePorts) carecontract.Intake {
	return application.NewIntake(postgres.NewStore(), ports, calendar.TodayDate)
}

// NewConclusion builds the conclusion at the acceptance (Weg b, #4680); it opens its own units.
func NewConclusion(contracts ContractPorts, intake IntakePorts) carecontract.Conclusion {
	clock := application.Clock{Now: time.Now}
	return application.NewConclusions(application.NewIntake(postgres.NewStore(), intake, calendar.TodayDate),
		application.NewContracts(postgres.NewStore(), rendering.NewRenderer(), contracts, clock, NewContractVersions()), clock)
}

// NewInbox builds the parents' inbox in the parents portal's Träger unit.
func NewInbox() carecontract.Inbox { return application.NewInbox(postgres.NewStore()) }

// NewContractMailRenderer renders a contract mail for Delivery's worker.
func NewContractMailRenderer() carecontract.ContractMailRenderer {
	return application.RenderContractMail
}
