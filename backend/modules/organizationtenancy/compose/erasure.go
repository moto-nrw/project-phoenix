package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/erasureledger"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/application"
	"github.com/uptrace/bun"
)

func NewErasure(db *bun.DB, files organizationtenancy.ErasureFiles, ledgerPath string) (organizationtenancy.Erasure, error) {
	store, err := postgres.NewErasureStore(db)
	if err != nil {
		return nil, err
	}
	return application.NewErasure(store, files, erasureledger.New(ledgerPath))
}

// InitializeErasureLedger is an explicit, exclusive provisioning step, never
// called by erase or restore. Existing files must never be silently replaced.
func InitializeErasureLedger(path string) error { return erasureledger.Initialize(path) }
