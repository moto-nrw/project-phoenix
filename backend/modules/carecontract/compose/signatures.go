package compose

import (
	"time"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/carecontract/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/carecontract/internal/adapters/rendering"
	"github.com/moto-ogs/project-phoenix/modules/carecontract/internal/application"
)

// SignaturePorts bind the signature of a contract (#4679).
type SignaturePorts = application.SignaturePorts

// NewSigning builds the one-time mail link; it opens its own Träger
// transaction.
func NewSigning(ports SignaturePorts) carecontract.Signing {
	return application.NewSignatures(postgres.NewStore(), ports, application.Clock{Now: time.Now}, nil)
}

// NewInboxSigning builds the signature in the parents portal, in the
// Träger transaction the parents portal opens.
func NewInboxSigning(ports SignaturePorts) carecontract.InboxSigning {
	return application.NewSignatures(postgres.NewStore(), ports, application.Clock{Now: time.Now}, nil)
}

// NewPaperSignatures builds the Träger-Büro's paper contracts. Writes use the
// route's Träger transaction; reads open their own units and log school access.
func NewPaperSignatures(contractPorts ContractPorts, ports SignaturePorts) carecontract.PaperSignatures {
	clock := application.Clock{Now: time.Now}
	store := postgres.NewStore()
	contracts := application.NewContracts(store, rendering.NewRenderer(), application.ContractPorts(contractPorts), clock, NewContractVersions())
	return application.NewSignatures(store, ports, clock, contracts)
}

type ReminderPorts struct {
	Organizations carecontract.TraegerOrganizations
	Settings      carecontract.ReminderSettings
	Sender        carecontract.TraegerSender
	Post          carecontract.Post
	Runtime       carecontract.Runtime
	Links         carecontract.SigningLinks
}

// NewReminders builds the reminder run of the Worker; it opens its own
// units.
func NewReminders(ports ReminderPorts) carecontract.Reminders {
	return application.NewReminders(postgres.NewStore(), application.ReminderPorts(ports), application.Clock{Now: time.Now})
}
