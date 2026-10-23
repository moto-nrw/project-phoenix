package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/application"
	"github.com/uptrace/bun"
)

func NewErasure(db *bun.DB, files organizationtenancy.ErasureFiles) (organizationtenancy.Erasure, error) {
	if db == nil || files == nil {
		return nil, errors.New("erasure: database and file store required")
	}
	store, err := postgres.NewErasureStore(db)
	if err != nil {
		return nil, err
	}
	return application.NewErasure(store, files), nil
}
