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

// NewContractVersions reads the versions in force and the recipients for the billing (#4681, #4684) in the caller's Träger unit.
func NewContractVersions() carecontract.ContractBilling {
	return application.NewBillingReads(postgres.NewStore())
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
func NewInbox() *application.Inbox { return application.NewInbox(postgres.NewStore()) }

// NewContractMailRenderer renders a contract mail for Delivery's worker.
func NewContractMailRenderer() carecontract.ContractMailRenderer {
	return application.RenderContractMail
}

// NewBookedCare builds the school's read of the booked care of its
// contracts, the Soll of Soll und Ist (#4683, ADR 0156). It reads in the
// tenant transaction of the school the caller holds.
func NewBookedCare() carecontract.BookedCare { return postgres.NewBookedCare() }

// NewOverrunEvaluations builds the evaluation of overruns over the schools
// of the Träger (#4683, ADR 0156); it opens its own units.
func NewOverrunEvaluations(ports OverrunPorts) carecontract.OverrunEvaluations {
	return application.NewOverruns(application.OverrunPorts(ports), calendar.TodayDate)
}

// OverrunPorts are the ports the root binds for the evaluation.
type OverrunPorts struct {
	Schools   carecontract.TraegerSchools
	CareTimes carecontract.SchoolCareTimes
	Access    carecontract.SchoolAccessLog
	Runtime   carecontract.Runtime
}
