package application

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/ports"
)

// Service answers room reads inside the caller's ambient transaction:
// facilities.rooms is tenant-scoped, so the caller's RLS context decides
// visibility and the service never opens an admin transaction of its own.
type Service struct {
	store        ports.Store
	tx           ports.Transaction
	lockDeletion ports.DeletionLock
	guard        ports.DeletionGuard
	observe      ports.Observer
}

func New(store ports.Store, tx ports.Transaction, lockDeletion ports.DeletionLock, guard ports.DeletionGuard, observe ports.Observer) *Service {
	if store == nil || tx == nil || lockDeletion == nil || guard == nil || observe == nil {
		panic("facilities application: all dependencies are required")
	}
	return &Service{store: store, tx: tx, lockDeletion: lockDeletion, guard: guard, observe: observe}
}

func (s *Service) Create(ctx context.Context, input domain.CreateRoom) (result domain.Room, err error) {
	err = s.runWrite(ctx, "create_room", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.lockRoomName(txCtx, input.Name); err != nil {
			return err
		}
		_, found, queryStats, findErr := s.store.FindByName(txCtx, input.Name)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if found {
			return domain.ErrDuplicate
		}
		result, queryStats, err = s.store.Create(txCtx, input)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func roomNameLock(name string) string {
	return "facilities:room-name:" + strings.ToLower(name)
}

func (s *Service) Update(ctx context.Context, input domain.UpdateRoom) (result domain.Room, err error) {
	err = s.runWrite(ctx, "update_room", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.lockRoomName(txCtx, input.Name); err != nil {
			return err
		}
		existing, found, queryStats, findErr := s.store.FindByID(txCtx, input.ID, "UPDATE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrNotFound
		}
		if err := validateRoomUpdate(existing, &input); err != nil {
			return err
		}
		if err := s.ensureUpdateNameAvailable(txCtx, existing, input, stats); err != nil {
			return err
		}
		result, queryStats, err = s.store.Update(txCtx, input)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func validateRoomUpdate(existing domain.Room, input *domain.UpdateRoom) error {
	if isSystemRoomName(existing.Name) && input.Name != existing.Name {
		return domain.ErrSystemProtected
	}
	if existing.Name != input.Name && strings.EqualFold(input.Name, domain.SchulhofRoomName) {
		return domain.ErrSystemNameReserved
	}
	if !isWCRoomName(existing.Name) {
		return nil
	}
	if input.Color != nil && !equalStringPointer(input.Color, existing.Color) {
		return domain.ErrSystemProtected
	}
	input.Color = existing.Color
	return nil
}

func (s *Service) ensureUpdateNameAvailable(
	ctx context.Context,
	existing domain.Room,
	input domain.UpdateRoom,
	stats *domain.OperationStats,
) error {
	if existing.Name == input.Name {
		return nil
	}
	duplicate, found, queryStats, err := s.store.FindByName(ctx, input.Name)
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	if found && duplicate.ID != input.ID {
		return domain.ErrDuplicate
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) (err error) {
	return s.runWrite(ctx, "delete_room", func(txCtx context.Context, stats *domain.OperationStats) error {
		// Timetable writers take the recurrence lock before room FK locks. Keep
		// deletion in the same order so concurrent template creation cannot
		// deadlock with a room delete.
		if err := s.lockDeletion(txCtx); err != nil {
			return err
		}
		existing, found, queryStats, findErr := s.store.FindByID(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrNotFound
		}
		if isSystemRoomName(existing.Name) {
			return domain.ErrSystemProtected
		}
		if err := s.guard(txCtx, id); err != nil {
			return err
		}
		queryStats, err = s.store.Delete(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func isSystemRoomName(name string) bool {
	return name == domain.SchulhofRoomName || name == domain.WCRoomName || name == domain.WCRoomAliasName
}

func isWCRoomName(name string) bool {
	return name == domain.WCRoomName || name == domain.WCRoomAliasName
}

func equalStringPointer(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func (s *Service) lockRoomName(ctx context.Context, name string) error {
	if err := s.tx.AcquireLock(ctx, roomNameLock(name)); err != nil {
		return err
	}
	if isWCRoomName(name) {
		return s.tx.AcquireLock(ctx, "facilities:room-name:wc-alias")
	}
	return nil
}

func (s *Service) FindByID(ctx context.Context, id int64) (result domain.Room, err error) {
	err = s.run("find_room", func(stats *domain.OperationStats) error {
		room, found, queryStats, findErr := s.store.FindByID(ctx, id, "")
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
	return result, err
}

func (s *Service) FindByIDForUpdate(ctx context.Context, id int64) (result domain.Room, err error) {
	err = s.run("find_room_for_update", func(stats *domain.OperationStats) error {
		room, found, queryStats, findErr := s.store.FindByID(ctx, id, "UPDATE")
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
	return result, err
}

func (s *Service) FindByName(ctx context.Context, name string) (result domain.Room, err error) {
	err = s.run("find_room_by_name", func(stats *domain.OperationStats) error {
		room, found, queryStats, findErr := s.store.FindByName(ctx, name)
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
	return result, err
}

func (s *Service) FindToilet(ctx context.Context, excludeRoomID int64) (result domain.Room, err error) {
	err = s.run("find_toilet_room", func(stats *domain.OperationStats) error {
		room, found, queryStats, findErr := s.store.FindToilet(ctx, excludeRoomID)
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
	return result, err
}

func (s *Service) List(ctx context.Context, filter domain.RoomFilter) (result []domain.Room, err error) {
	err = s.run("list_rooms", func(stats *domain.OperationStats) error {
		rooms, queryStats, listErr := s.store.List(ctx, filter)
		stats.Add(queryStats)
		result = rooms
		return listErr
	})
	return result, err
}

func (s *Service) ListByIDs(ctx context.Context, ids []int64) (result []domain.Room, err error) {
	err = s.run("list_rooms_by_id", func(stats *domain.OperationStats) error {
		rooms, queryStats, listErr := s.store.ListByIDs(ctx, ids)
		stats.Add(queryStats)
		result = rooms
		return listErr
	})
	return result, err
}

func (s *Service) LockByIDs(ctx context.Context, ids []int64) (result []domain.Room, err error) {
	err = s.run("lock_rooms_by_id", func(stats *domain.OperationStats) error {
		rooms, queryStats, lockErr := s.store.LockByIDs(ctx, ids)
		stats.Add(queryStats)
		result = rooms
		return lockErr
	})
	return result, err
}

func (s *Service) run(operation string, fn func(*domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = fn(&stats)
	if err != nil {
		stats.Rows = 0
	}
	return err
}

func (s *Service) runWrite(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = s.tx.RunWrite(ctx, func(txCtx context.Context) error { return fn(txCtx, &stats) })
	if err != nil {
		stats.Rows = 0
	}
	return err
}
