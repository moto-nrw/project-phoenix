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
	usage        ports.RoomUsage
	observe      ports.Observer
}

func New(store ports.Store, tx ports.Transaction, lockDeletion ports.DeletionLock, usage ports.RoomUsage, observe ports.Observer) *Service {
	if lockDeletion == nil || usage == nil {
		panic("facilities application: all dependencies are required")
	}
	service := NewRooms(store, tx, observe)
	service.lockDeletion, service.usage = lockDeletion, usage
	return service
}

// NewRooms wires the service for every room operation except Delete, which
// needs the deletion lock and guard of New. Only facilities.Rooms, which has
// no deletion, may serve it.
func NewRooms(store ports.Store, tx ports.Transaction, observe ports.Observer) *Service {
	if store == nil || tx == nil || observe == nil {
		panic("facilities application: all dependencies are required")
	}
	return &Service{store: store, tx: tx, observe: observe}
}

func (s *Service) Create(ctx context.Context, input domain.CreateRoom) (result domain.Room, err error) {
	err = s.runWrite(ctx, "create_room", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.lockRoomName(txCtx, input.Name); err != nil {
			return err
		}
		existing, found, queryStats, findErr := s.store.FindByName(txCtx, input.Name)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if found {
			return duplicateRoomError(input.Name, existing.Name)
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
		return duplicateRoomError(input.Name, duplicate.Name)
	}
	return nil
}

func duplicateRoomError(name, duplicate string) error {
	if isWCRoomName(name) && isWCRoomName(duplicate) {
		return domain.ErrDuplicateToilet
	}
	return domain.ErrDuplicate
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
		if err := s.refuseUsedRoom(txCtx, id); err != nil {
			return err
		}
		queryStats, err = s.store.Delete(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

// refuseUsedRoom keeps a room while a session runs in it or a care offering
// needs it. The running session is asked first.
func (s *Service) refuseUsedRoom(ctx context.Context, id int64) error {
	active, err := s.usage.HasActiveSessions(ctx, id)
	if err != nil {
		return err
	}
	if active {
		return domain.ErrInUse
	}
	required, err := s.usage.RequiredByCareOffering(ctx, id)
	if err != nil {
		return err
	}
	if required {
		return domain.ErrRequiredByOffering
	}
	return nil
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
	err = s.runWrite(ctx, "find_room", func(txCtx context.Context, stats *domain.OperationStats) error {
		room, found, queryStats, findErr := s.store.FindByID(txCtx, id, "")
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
	err = s.runWrite(ctx, "find_toilet_room", func(txCtx context.Context, stats *domain.OperationStats) error {
		room, found, queryStats, findErr := s.store.FindToilet(txCtx, excludeRoomID)
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
