package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
}

// New composes the Facilities module. Reads run on the caller's ambient
// tenant transaction when one exists (tenant middleware, scheduler loops)
// and otherwise on the shared connection, exactly like the legacy
// repositories' base.GetDB resolution, so RLS visibility is unchanged.
func New(dependencies Dependencies) (*facilities.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("facilities compose: all dependencies are required")
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
			return nil, fmt.Errorf("facilities postgres: unsupported transaction %T", transaction)
		}
	})
	service := application.New(store, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	})
	return facilities.NewModule(engine{service: service}), nil
}

type engine struct{ service *application.Service }

func (e engine) FindRoom(ctx context.Context, id int64) (facilities.Room, error) {
	value, err := e.service.FindByID(ctx, id)
	return toPublic(value), mapError(err)
}

func (e engine) ListRoomsByID(ctx context.Context, ids []int64) ([]facilities.Room, error) {
	values, err := e.service.ListByIDs(ctx, ids)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]facilities.Room, 0, len(values))
	for _, value := range values {
		result = append(result, toPublic(value))
	}
	return result, nil
}

func toPublic(value domain.Room) facilities.Room {
	return facilities.Room{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Name: value.Name, Building: value.Building, Floor: value.Floor, Capacity: value.Capacity,
		Category: value.Category, Color: value.Color, IsSystem: value.IsSystem,
	}
}

func mapError(err error) error {
	if errors.Is(err, domain.ErrNotFound) {
		return facilities.ErrRoomNotFound
	}
	return err
}
