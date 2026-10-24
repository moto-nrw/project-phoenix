package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/adapters/postgres"
	application "github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/ports"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
}

// New composes the School Structure service, which is the whole Owner
// surface, over the caller's authoritative transaction.
func New(dependencies Dependencies) (schoolstructure.Owner, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("school structure compose: all dependencies are required")
	}
	store := postgres.New(postgres.NewAmbientDatabase(tenant.TransactionFromContext))
	return application.New(store, application.NewTransitionRetention(store), dependencies.Observe), nil
}
