package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/application"
	"github.com/uptrace/bun"
)

func NewErasure(db *bun.DB, files organizationtenancy.ErasureFiles, ledger organizationtenancy.ErasureLedger) (organizationtenancy.Erasure, error) {
	store, err := postgres.NewErasureStore(db)
	if err != nil {
		return nil, err
	}
	return application.NewErasure(store, files, ledger)
}

// NewDemoSchoolFiles composes the file erasure of retired demo schools (#5544).
func NewDemoSchoolFiles(db *bun.DB, files organizationtenancy.ErasureFiles) (organizationtenancy.DemoSchoolFiles, error) {
	if db == nil {
		return nil, errors.New("demo school files require a database")
	}
	return application.NewDemoSchoolFiles(postgres.NewDemoSchoolFileStore(db), files)
}
