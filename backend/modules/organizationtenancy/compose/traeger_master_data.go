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

// NewTraegerDocuments builds the Träger-Büro's shared documents (#4692).
func NewTraegerDocuments(dependencies organizationtenancy.TraegerDocumentPorts) organizationtenancy.TraegerDocuments {
	return application.NewTraegerDocuments(postgres.NewTraegerDocumentStore(), dependencies)
}

// NewSharedTraegerDocuments builds the parents' and schools' reads (#4692).
func NewSharedTraegerDocuments() *postgres.TraegerDocumentStore {
	return postgres.NewTraegerDocumentStore()
}

// NewReleasePushValidity checks current release identity and audience in batch.
func NewReleasePushValidity() organizationtenancy.ReleasePushValidity {
	return application.NewReleasePushValidity(postgres.NewTraegerDocumentStore())
}
