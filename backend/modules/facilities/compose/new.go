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
	DB            *bun.DB
	DeletionLock  func(context.Context) error
	DeletionGuard func(context.Context, int64) error
	Observe       func(Observation)
}

// New composes the Facilities module. Reads run on the caller's ambient
// tenant transaction when one exists (tenant middleware, scheduler loops).
// Shared connections require a caller tenant and apply an explicit predicate;
// an ambient transaction without one relies on its RLS scope (device auth).
func New(dependencies Dependencies) (*facilities.Module, error) {
	if dependencies.DB == nil || dependencies.DeletionLock == nil || dependencies.DeletionGuard == nil || dependencies.Observe == nil {
		return nil, errors.New("facilities compose: all dependencies are required")
	}
	store := postgres.New(databaseRuntime(dependencies.DB))
	service := application.New(store, transaction{}, dependencies.DeletionLock, dependencies.DeletionGuard, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	})
	return facilities.NewModule(engine{service: service}), nil
}

func databaseRuntime(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID, tenantErr := tenant.TenantFromContext(ctx)
		id := int64(0)
		if tenantErr == nil {
			id = tenantID.Int64()
		}
		transaction, hasTransaction := tenant.TransactionFromContext(ctx)
		if !hasTransaction {
			if tenantErr != nil {
				return nil, 0, fmt.Errorf("facilities postgres: tenant is required: %w", tenantErr)
			}
			return db, id, nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, id, nil
		case *bun.Tx:
			if tx != nil {
				return tx, id, nil
			}
			if tenantErr != nil {
				return nil, 0, fmt.Errorf("facilities postgres: tenant is required: %w", tenantErr)
			}
			return db, id, nil
		default:
			return nil, 0, fmt.Errorf("facilities postgres: unsupported transaction %T", transaction)
		}
	}
}

type engine struct{ service *application.Service }

type transaction struct{}

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return tenant.WithinCurrentTenant(ctx, callback)
}

func (transaction) AcquireLock(ctx context.Context, key string) error {
	return tenant.AcquireLock(ctx, key, false)
}

func (e engine) CreateRoom(ctx context.Context, input facilities.CreateRoom) (facilities.Room, error) {
	value, err := e.service.Create(ctx, domain.CreateRoom{
		Name: input.Name, Building: input.Building, Floor: input.Floor, Capacity: input.Capacity,
		Category: input.Category, Color: input.Color, IsSystem: input.IsSystem,
	})
	return toPublic(value), mapError(err)
}

func (e engine) UpdateRoom(ctx context.Context, input facilities.UpdateRoom) (facilities.Room, error) {
	value, err := e.service.Update(ctx, domain.UpdateRoom{
		ID: input.ID, Name: input.Name, Building: input.Building, Floor: input.Floor,
		Capacity: input.Capacity, Category: input.Category, Color: input.Color,
	})
	return toPublic(value), mapError(err)
}

func (e engine) DeleteRoom(ctx context.Context, id int64) error {
	return mapError(e.service.Delete(ctx, id))
}

func (e engine) FindRoom(ctx context.Context, id int64) (facilities.Room, error) {
	value, err := e.service.FindByID(ctx, id)
	return toPublic(value), mapError(err)
}

func (e engine) FindRoomForUpdate(ctx context.Context, id int64) (facilities.Room, error) {
	value, err := e.service.FindByIDForUpdate(ctx, id)
	return toPublic(value), mapError(err)
}

func (e engine) FindRoomByName(ctx context.Context, name string) (facilities.Room, error) {
	value, err := e.service.FindByName(ctx, name)
	return toPublic(value), mapError(err)
}

func (e engine) FindToiletRoom(ctx context.Context, excludeRoomID int64) (facilities.Room, error) {
	value, err := e.service.FindToilet(ctx, excludeRoomID)
	return toPublic(value), mapError(err)
}

func (e engine) ListRooms(ctx context.Context, filter facilities.RoomFilter) ([]facilities.Room, error) {
	values, err := e.service.List(ctx, domain.RoomFilter{
		Name: filter.Name, NameContains: filter.NameContains, Building: filter.Building,
		BuildingContains: filter.BuildingContains, Floor: filter.Floor, Category: filter.Category,
		MinimumCapacity: filter.MinimumCapacity, MaximumCapacity: filter.MaximumCapacity, Search: filter.Search,
		ExcludeSystem: filter.ExcludeSystem,
	})
	return toPublicList(values), mapError(err)
}

func (e engine) ListRoomsPage(ctx context.Context, filter facilities.RoomFilter, offset, limit int) (facilities.RoomPage, error) {
	values, total, err := e.service.ListPage(ctx, domain.RoomFilter{
		Name: filter.Name, NameContains: filter.NameContains, Building: filter.Building,
		BuildingContains: filter.BuildingContains, Floor: filter.Floor, Category: filter.Category,
		MinimumCapacity: filter.MinimumCapacity, MaximumCapacity: filter.MaximumCapacity, Search: filter.Search,
		ExcludeSystem: filter.ExcludeSystem,
	}, offset, limit)
	return facilities.RoomPage{Rooms: toPublicList(values), Total: total}, mapError(err)
}

func (e engine) ListRoomsByID(ctx context.Context, ids []int64) ([]facilities.Room, error) {
	values, err := e.service.ListByIDs(ctx, ids)
	return toPublicList(values), mapError(err)
}

func (e engine) LockRoomsByID(ctx context.Context, ids []int64) ([]facilities.Room, error) {
	values, err := e.service.LockByIDs(ctx, ids)
	return toPublicList(values), mapError(err)
}

func toPublicList(values []domain.Room) []facilities.Room {
	result := make([]facilities.Room, 0, len(values))
	for _, value := range values {
		result = append(result, toPublic(value))
	}
	return result
}

func toPublic(value domain.Room) facilities.Room {
	return facilities.Room{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Name: value.Name, Building: value.Building, Floor: value.Floor, Capacity: value.Capacity,
		Category: value.Category, Color: value.Color, IsSystem: value.IsSystem,
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return facilities.ErrRoomNotFound
	case errors.Is(err, domain.ErrDuplicate):
		return facilities.ErrDuplicateRoom
	case errors.Is(err, domain.ErrDuplicateToilet):
		return facilities.ErrDuplicateToiletRoom
	case errors.Is(err, domain.ErrColorAlreadyInUse):
		return facilities.ErrRoomColorAlreadyInUse
	case errors.Is(err, domain.ErrSystemProtected):
		return facilities.ErrSystemRoomProtected
	case errors.Is(err, domain.ErrSystemNameReserved):
		return facilities.ErrSystemRoomNameReserved
	default:
		return err
	}
}
