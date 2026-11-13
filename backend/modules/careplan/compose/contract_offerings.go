package compose

import (
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/adapters/postgres"
)

// NewContractOfferings reads a school's offerings for the Träger's contracts (#4678, ADR 0153) in that school's tenant transaction.
func NewContractOfferings() careplan.ContractOfferings { return postgres.NewContractOfferingStore() }
