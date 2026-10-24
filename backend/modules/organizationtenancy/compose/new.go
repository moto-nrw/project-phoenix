package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
}

func New(dependencies Dependencies) (organizationtenancy.Capability, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("organization tenancy compose: all dependencies are required")
	}
	store := postgres.New(postgres.NewDatabase())
	return application.New(store, postgres.Transaction{}, dependencies.Observe), nil
}
