package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/application"
)

// NewTraegerMasterData builds the master data of a Träger (#4653, ADR 0112).
// It runs in the Träger transaction the Träger-Portal route opens (ADR 0105).
func NewTraegerMasterData() organizationtenancy.TraegerMasterData {
	return application.NewTraegerMasterData(postgres.NewTraegerMasterDataStore())
}

// NewTraegerDocumentTemplates builds the document templates of a Träger
// (#4677, ADR 0152) in the Träger transaction the route opens.
func NewTraegerDocumentTemplates() organizationtenancy.TraegerDocumentTemplates {
	return application.NewTraegerDocumentTemplates(postgres.NewTraegerMasterDataStore())
}

// NewTraegerDocuments builds the shared documents of the Träger-Büro
// (#4692). The service opens the Träger and school transactions itself
// through the runtime the root binds.
func NewTraegerDocuments(dependencies organizationtenancy.TraegerDocumentPorts) organizationtenancy.TraegerDocuments {
	return application.NewTraegerDocuments(postgres.NewTraegerDocumentStore(), dependencies)
}

// SharedTraegerDocuments is what parents and schools read of the shared
// documents, in the transaction their caller opens.
type SharedTraegerDocuments interface {
	organizationtenancy.ParentTraegerDocuments
	organizationtenancy.SchoolTraegerDocuments
}

// NewSharedTraegerDocuments builds the parents' and the schools' reads of
// the shared documents (#4692).
func NewSharedTraegerDocuments() SharedTraegerDocuments {
	return application.NewSharedTraegerDocuments(postgres.NewTraegerDocumentStore())
}
