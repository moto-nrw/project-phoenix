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

// New composes School Structure over the caller's authoritative transaction.
func New(dependencies Dependencies) (*schoolstructure.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("school structure compose: all dependencies are required")
	}
	store := postgres.New(postgres.NewAmbientDatabase(tenant.TransactionFromContext))
	service := application.New(store, application.NewObserver(dependencies.Observe))
	return schoolstructure.NewModule(application.NewEngine(service), application.NewTransitionRetention(store)), nil
}
