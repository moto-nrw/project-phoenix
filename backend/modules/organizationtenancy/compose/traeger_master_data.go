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
// (#4677, ADR 0149) in the Träger transaction the route opens.
func NewTraegerDocumentTemplates() organizationtenancy.TraegerDocumentTemplates {
	return application.NewTraegerDocumentTemplates(postgres.NewTraegerMasterDataStore())
}
