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

// NewDemoSchoolErasure composes the file (#5544) and database (#5559) erasure of retired demo schools.
func NewDemoSchoolErasure(db *bun.DB, files organizationtenancy.ErasureFiles) (organizationtenancy.DemoSchoolErasure, error) {
	if db == nil {
		return nil, errors.New("demo school erasure requires a database")
	}
	return application.NewDemoSchoolErasure(postgres.NewDemoSchoolFileStore(db), postgres.NewDemoSchoolDatabaseStore(db), files)
}
