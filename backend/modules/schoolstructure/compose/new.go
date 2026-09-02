package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
}

// New composes the School Structure module. Reads run on the caller's
// ambient tenant transaction when one exists (tenant middleware, scheduler
// loops) and otherwise on the shared connection, exactly like the legacy
// repositories' base.GetDB resolution, so RLS visibility is unchanged.
func New(dependencies Dependencies) (*schoolstructure.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("school structure compose: all dependencies are required")
	}
	store := postgres.New(func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return dependencies.DB, nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, nil
		case *bun.Tx:
			if tx != nil {
				return tx, nil
			}
			return dependencies.DB, nil
		default:
			return nil, fmt.Errorf("school structure postgres: unsupported transaction %T", transaction)
		}
	})
	service := application.New(store, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	})
	return schoolstructure.NewModule(engine{service: service}), nil
}

type engine struct{ service *application.Service }

func (e engine) FindGroupByID(ctx context.Context, id int64) (schoolstructure.Group, error) {
	value, err := e.service.FindByID(ctx, id)
	return toPublic(value), mapError(err)
}

func (e engine) ListGroupsByID(ctx context.Context, ids []int64) ([]schoolstructure.Group, error) {
	values, err := e.service.ListByIDs(ctx, ids)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]schoolstructure.Group, 0, len(values))
	for _, value := range values {
		result = append(result, toPublic(value))
	}
	return result, nil
}

func toPublic(value domain.Group) schoolstructure.Group {
	return schoolstructure.Group{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Name: value.Name, RoomID: value.RoomID,
	}
}

func mapError(err error) error {
	if errors.Is(err, domain.ErrNotFound) {
		return schoolstructure.ErrGroupNotFound
	}
	return err
}
