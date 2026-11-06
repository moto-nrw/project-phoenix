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
	return application.NewContracts(postgres.NewStore(), rendering.NewRenderer(), application.ContractPorts(ports), application.Clock{Now: time.Now})
}

// ContractPorts are the ports the root binds for the contracts.
type ContractPorts struct {
	Letterheads carecontract.Letterheads
	Post        carecontract.Post
	Runtime     carecontract.Runtime
	Access      carecontract.SchoolAccessLog
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

// NewBookedCare builds the school's read of the booked care of its
// contracts, the Soll of Soll und Ist (#4683, ADR 0155). It reads in the
// tenant transaction of the school the caller holds.
func NewBookedCare() carecontract.BookedCare { return postgres.NewBookedCare() }

// NewOverrunEvaluations builds the evaluation of overruns over the schools
// of the Träger (#4683, ADR 0155); it opens its own units.
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
