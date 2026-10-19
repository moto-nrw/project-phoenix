package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/ports"
)

// Engine translates the owner contract and runs its observed room operations.
type Engine struct {
	service *Service
	observe ports.Observer
}

func NewEngine(service *Service, observe ports.Observer) Engine {
	return Engine{service: service, observe: observe}
}

func (e Engine) ObserveRejection(operation string, duration time.Duration, err error) {
	e.observe(ports.Observation{Operation: operation, Duration: duration, Err: err})
}

func (e Engine) CreateRoom(ctx context.Context, input facilities.CreateRoom) (facilities.Room, error) {
	value, err := e.service.Create(ctx, domain.CreateRoom{
		Name: input.Name, Building: input.Building, Floor: input.Floor, Capacity: input.Capacity,
		Category: input.Category, Color: input.Color, IsSystem: input.IsSystem,
		IsOpenRoom: input.IsOpenRoom,
	})
	return toPublic(value), MapError(err)
}

func (e Engine) UpdateRoom(ctx context.Context, input facilities.UpdateRoom) (facilities.Room, error) {
	value, err := e.service.Update(ctx, domain.UpdateRoom{
		ID: input.ID, Name: input.Name, Building: input.Building, Floor: input.Floor,
		Capacity: input.Capacity, Category: input.Category, Color: input.Color,
		IsOpenRoom: input.IsOpenRoom,
	})
	return toPublic(value), MapError(err)
}

func (e Engine) DeleteRoom(ctx context.Context, id int64) error {
	return MapError(e.service.Delete(ctx, id))
}

func (e Engine) FindRoom(ctx context.Context, id int64) (facilities.Room, error) {
	value, err := e.service.FindByID(ctx, id)
	return toPublic(value), MapError(err)
}

func (e Engine) FindRoomForUpdate(ctx context.Context, id int64) (facilities.Room, error) {
	var result domain.Room
	err := e.service.run("find_room_for_update", func(stats *domain.OperationStats) error {
		room, found, queryStats, findErr := e.service.store.FindByID(ctx, id, "UPDATE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrNotFound
		}
		result = room
		return nil
	})
	return toPublic(result), MapError(err)
}

func (e Engine) FindRoomByName(ctx context.Context, name string) (facilities.Room, error) {
	var result domain.Room
	err := e.service.runWrite(ctx, "find_room_by_name", func(ctx context.Context, stats *domain.OperationStats) error {
		room, found, queryStats, findErr := e.service.store.FindByName(ctx, name)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrNotFound
		}
		result = room
		return nil
	})
	return toPublic(result), MapError(err)
}

func (e Engine) FindToiletRoom(ctx context.Context, excludeRoomID int64) (facilities.Room, error) {
	value, err := e.service.FindToilet(ctx, excludeRoomID)
	return toPublic(value), MapError(err)
}

func (e Engine) ListRooms(ctx context.Context, filter facilities.RoomFilter) ([]facilities.Room, error) {
	var result []domain.Room
	var err error
	domainFilter := domain.RoomFilter{
		Name: filter.Name, NameContains: filter.NameContains, Building: filter.Building,
		BuildingContains: filter.BuildingContains, Floor: filter.Floor, Category: filter.Category,
		MinimumCapacity: filter.MinimumCapacity, MaximumCapacity: filter.MaximumCapacity, Search: filter.Search,
		ExcludeSystem: filter.ExcludeSystem, IsOpenRoom: filter.IsOpenRoom,
	}

	err = e.service.runWrite(ctx, "list_rooms", func(ctx context.Context, stats *domain.OperationStats) error {
		rooms, queryStats, listErr := e.service.store.List(ctx, domainFilter)
		stats.Add(queryStats)
		result = rooms
		return listErr
	})
	return toPublicList(result), MapError(err)
}

func (e Engine) ListRoomsPage(ctx context.Context, filter facilities.RoomFilter, offset, limit int) (facilities.RoomPage, error) {
	var result []domain.Room
	var err error
	var total int
	domainFilter := domain.RoomFilter{
		Name: filter.Name, NameContains: filter.NameContains, Building: filter.Building,
		BuildingContains: filter.BuildingContains, Floor: filter.Floor, Category: filter.Category,
		MinimumCapacity: filter.MinimumCapacity, MaximumCapacity: filter.MaximumCapacity, Search: filter.Search,
		ExcludeSystem: filter.ExcludeSystem, IsOpenRoom: filter.IsOpenRoom,
	}

	err = e.service.runWrite(ctx, "list_rooms_page", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, total, queryStats, err = e.service.store.ListPage(ctx, domainFilter, offset, limit)
		stats.Add(queryStats)
		return err
	})
	return facilities.RoomPage{Rooms: toPublicList(result), Total: total}, MapError(err)
}

func (e Engine) ListRoomsByID(ctx context.Context, ids []int64) ([]facilities.Room, error) {
	var result []domain.Room
	err := e.service.runWrite(ctx, "list_rooms_by_id", func(ctx context.Context, stats *domain.OperationStats) error {
		rooms, queryStats, listErr := e.service.store.ListByIDs(ctx, ids)
		stats.Add(queryStats)
		result = rooms
		return listErr
	})
	return toPublicList(result), MapError(err)
}

func (e Engine) LockRoomsByID(ctx context.Context, ids []int64) ([]facilities.Room, error) {
	var result []domain.Room
	err := e.service.run("lock_rooms_by_id", func(stats *domain.OperationStats) error {
		rooms, queryStats, lockErr := e.service.store.LockByIDs(ctx, ids)
		stats.Add(queryStats)
		result = rooms
		return lockErr
	})
	return toPublicList(result), MapError(err)
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
		IsOpenRoom: value.IsOpenRoom,
	}
}

func MapError(err error) error {
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
	case errors.Is(err, domain.ErrInUse):
		return facilities.ErrRoomInUse
	case errors.Is(err, domain.ErrRequiredByOffering):
		return facilities.ErrRoomRequiredByOffering
	default:
		return err
	}
}
