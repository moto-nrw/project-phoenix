package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/application"
)

// ChangePorts are the ports the root binds for the office's decision of
// the parents' changes (#5979).
type ChangePorts = application.ChangePorts

// NewContractChanges builds the office's decision of the parents' changes
// (#5979); it opens its own units.
func NewContractChanges(ports ChangePorts) carecontract.ContractChangeDecisions {
	return application.NewContractChanges(postgres.NewStore(), ports)
}
