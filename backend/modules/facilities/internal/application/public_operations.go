package application

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (s *Service) CreateRoom(ctx context.Context, input facilities.CreateRoom) (result facilities.Room, err error) {
	started := time.Now()
	if err := s.requireTenant(ctx, "create_room"); err != nil {
		return facilities.Room{}, err
	}
	if err := normalizeAndValidateCreateRoom(&input); err != nil {
		s.ObserveRejection("create_room", time.Since(started), err)
		return facilities.Room{}, err
	}
	if strings.EqualFold(input.Name, facilities.SchulhofRoomName) && (input.Name != facilities.SchulhofRoomName || !input.IsSystem) {
		err := facilities.ErrSystemRoomNameReserved
		s.ObserveRejection("create_room", time.Since(started), err)
		return facilities.Room{}, err
	}
	if input.IsOpenRoom && facilities.IsToiletRoomName(input.Name) {
		err := facilities.ErrToiletRoomNotReleasable
		s.ObserveRejection("create_room", time.Since(started), err)
		return facilities.Room{}, err
	}
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
func (s *Service) UpdateRoom(ctx context.Context, input facilities.UpdateRoom) (result facilities.Room, err error) {
	started := time.Now()
	if err := s.requireTenant(ctx, "update_room"); err != nil {
		return facilities.Room{}, err
	}
	if input.ID <= 0 {
		err := &facilities.InvalidRoomError{Reason: "room ID is required"}
		s.ObserveRejection("update_room", time.Since(started), err)
		return facilities.Room{}, err
	}
	if err := normalizeAndValidateRoom(&input.Name, input.Capacity, &input.Color); err != nil {
		s.ObserveRejection("update_room", time.Since(started), err)
		return facilities.Room{}, err
	}
	// Checked against the submitted name: a toilet room cannot be renamed
	// (ErrSystemRoomProtected covers that), so the name reaching this point
	// is the room's real one.
	if input.IsOpenRoom != nil && *input.IsOpenRoom && facilities.IsToiletRoomName(input.Name) {
		err := facilities.ErrToiletRoomNotReleasable
		s.ObserveRejection("update_room", time.Since(started), err)
		return facilities.Room{}, err
	}
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
			return facilities.ErrRoomNotFound
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
func (s *Service) DeleteRoom(ctx context.Context, id int64) (err error) {
	if err := s.requireTenant(ctx, "delete_room"); err != nil {
		return err
	}
	if id <= 0 {
		err := &facilities.InvalidRoomError{Reason: "room ID is required"}
		s.ObserveRejection("delete_room", 0, err)
		return err
	}
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
			return facilities.ErrRoomNotFound
		}
		if isSystemRoomName(existing.Name) {
			return facilities.ErrSystemRoomProtected
		}
		if err := s.refuseUsedRoom(txCtx, id); err != nil {
			return err
		}
		queryStats, err = s.store.Delete(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}
func (s *Service) FindRoom(ctx context.Context, id int64) (result facilities.Room, err error) {
	if err := s.requireTenant(ctx, "find_room"); err != nil {
		return facilities.Room{}, err
	}
	if id <= 0 {
		err := facilities.ErrInvalidRoom
		s.ObserveRejection("find_room", 0, err)
		return facilities.Room{}, err
	}
	err = s.runWrite(ctx, "find_room", func(txCtx context.Context, stats *domain.OperationStats) error {
		room, found, queryStats, findErr := s.store.FindByID(txCtx, id, "")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return facilities.ErrRoomNotFound
		}
		result = room
		return nil
	})
	return result, err
}
func (s *Service) FindToiletRoom(ctx context.Context, excludeRoomID int64) (result facilities.Room, err error) {
	if err := s.requireTenant(ctx, "find_toilet_room"); err != nil {
		return facilities.Room{}, err
	}
	if excludeRoomID < 0 {
		err := &facilities.InvalidRoomError{Reason: "excluded room ID cannot be negative"}
		s.ObserveRejection("find_toilet_room", 0, err)
		return facilities.Room{}, err
	}
	err = s.runWrite(ctx, "find_toilet_room", func(txCtx context.Context, stats *domain.OperationStats) error {
		room, found, queryStats, findErr := s.store.FindToilet(txCtx, excludeRoomID)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return facilities.ErrRoomNotFound
		}
		result = room
		return nil
	})
	return result, err
}
func (s *Service) FindRoomForUpdate(ctx context.Context, id int64) (facilities.Room, error) {
	if err := s.requireTenant(ctx, "find_room_for_update"); err != nil {
		return facilities.Room{}, err
	}
	if id <= 0 {
		err := facilities.ErrInvalidRoom
		s.ObserveRejection("find_room_for_update", 0, err)
		return facilities.Room{}, err
	}
	var result facilities.Room
	err := s.run("find_room_for_update", func(stats *domain.OperationStats) error {
		room, found, queryStats, findErr := s.store.FindByID(ctx, id, "UPDATE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return facilities.ErrRoomNotFound
		}
		result = room
		return nil
	})
	return result, err
}
func (s *Service) FindRoomByName(ctx context.Context, name string) (facilities.Room, error) {
	if err := s.requireTenant(ctx, "find_room_by_name"); err != nil {
		return facilities.Room{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		err := &facilities.InvalidRoomError{Reason: "room name is required"}
		s.ObserveRejection("find_room_by_name", 0, err)
		return facilities.Room{}, err
	}
	var result facilities.Room
	err := s.runWrite(ctx, "find_room_by_name", func(ctx context.Context, stats *domain.OperationStats) error {
		room, found, queryStats, findErr := s.store.FindByName(ctx, name)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return facilities.ErrRoomNotFound
		}
		result = room
		return nil
	})
	return result, err
}
func (s *Service) ListRooms(ctx context.Context, filter facilities.RoomFilter) ([]facilities.Room, error) {
	if err := s.requireTenant(ctx, "list_rooms"); err != nil {
		return nil, err
	}
	if filter.MinimumCapacity != nil && *filter.MinimumCapacity < 0 {
		err := &facilities.InvalidRoomError{Reason: "minimum capacity cannot be negative"}
		s.ObserveRejection("list_rooms", 0, err)
		return nil, err
	}
	if filter.MaximumCapacity != nil && *filter.MaximumCapacity < 0 {
		err := &facilities.InvalidRoomError{Reason: "maximum capacity cannot be negative"}
		s.ObserveRejection("list_rooms", 0, err)
		return nil, err
	}
	var result []facilities.Room
	var err error
	domainFilter := filter

	err = s.runWrite(ctx, "list_rooms", func(ctx context.Context, stats *domain.OperationStats) error {
		rooms, queryStats, listErr := s.store.List(ctx, domainFilter)
		stats.Add(queryStats)
		result = rooms
		return listErr
	})
	return nonNilRooms(result), err
}
func (s *Service) ListRoomsPage(ctx context.Context, filter facilities.RoomFilter, offset, limit int) (facilities.RoomPage, error) {
	if err := s.requireTenant(ctx, "list_rooms_page"); err != nil {
		return facilities.RoomPage{}, err
	}
	if offset < 0 || limit <= 0 {
		err := &facilities.InvalidRoomError{Reason: "room page is invalid"}
		s.ObserveRejection("list_rooms_page", 0, err)
		return facilities.RoomPage{}, err
	}
	var result []facilities.Room
	var err error
	var total int
	domainFilter := filter

	err = s.runWrite(ctx, "list_rooms_page", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, total, queryStats, err = s.store.ListPage(ctx, domainFilter, offset, limit)
		stats.Add(queryStats)
		return err
	})
	return facilities.RoomPage{Rooms: nonNilRooms(result), Total: total}, err
}
func (s *Service) ListRoomsByID(ctx context.Context, ids []int64) ([]facilities.Room, error) {
	if err := s.requireTenant(ctx, "list_rooms_by_id"); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []facilities.Room{}, nil
	}
	for _, id := range ids {
		if id <= 0 {
			err := facilities.ErrInvalidRoom
			s.ObserveRejection("list_rooms_by_id", 0, err)
			return nil, err
		}
	}
	var result []facilities.Room
	err := s.runWrite(ctx, "list_rooms_by_id", func(ctx context.Context, stats *domain.OperationStats) error {
		rooms, queryStats, listErr := s.store.ListByIDs(ctx, ids)
		stats.Add(queryStats)
		result = rooms
		return listErr
	})
	return nonNilRooms(result), err
}
func (s *Service) LockRoomsByID(ctx context.Context, ids []int64) ([]facilities.Room, error) {
	if err := s.requireTenant(ctx, "lock_rooms_by_id"); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []facilities.Room{}, nil
	}
	for _, id := range ids {
		if id <= 0 {
			err := facilities.ErrInvalidRoom
			s.ObserveRejection("lock_rooms_by_id", 0, err)
			return nil, err
		}
	}
	var result []facilities.Room
	err := s.run("lock_rooms_by_id", func(stats *domain.OperationStats) error {
		rooms, queryStats, lockErr := s.store.LockByIDs(ctx, ids)
		stats.Add(queryStats)
		result = rooms
		return lockErr
	})
	return nonNilRooms(result), err
}
func (s *Service) requireTenant(ctx context.Context, operation string) error {
	_, err := tenant.TenantFromContext(ctx)
	if err != nil {
		s.ObserveRejection(operation, 0, err)
	}
	return err
}
func (s *Service) ObserveRejection(operation string, duration time.Duration, err error) {
	s.observe(ports.Observation{Operation: operation, Duration: duration, Err: err})
}
func nonNilRooms(rooms []facilities.Room) []facilities.Room {
	if rooms == nil {
		return []facilities.Room{}
	}
	return rooms
}

var roomColorPattern = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

func normalizeAndValidateCreateRoom(input *facilities.CreateRoom) error {
	return normalizeAndValidateRoom(&input.Name, input.Capacity, &input.Color)
}
func normalizeAndValidateRoom(name *string, capacity *int, color **string) error {
	*name = strings.TrimSpace(*name)
	if *name == "" {
		return &facilities.InvalidRoomError{Reason: "room name is required"}
	}
	if capacity != nil && *capacity <= 0 {
		return &facilities.InvalidRoomError{Reason: "capacity must be greater than zero"}
	}
	if *color == nil || strings.TrimSpace(**color) == "" {
		return nil
	}
	normalized, err := normalizeColor(**color)
	if err != nil {
		return err
	}
	*color = &normalized
	return nil
}

// normalizeColor turns a hex color into the stored #RRGGBB form and refuses
// the colors reserved for status badges. Room and category colors share it:
// both end up on the same presence badge.
func normalizeColor(raw string) (string, error) {
	normalized := strings.TrimSpace(raw)
	if !strings.HasPrefix(normalized, "#") {
		normalized = "#" + normalized
	}
	if !roomColorPattern.MatchString(normalized) {
		return "", &facilities.InvalidRoomError{Reason: "invalid color format, must be a valid hex color"}
	}
	if len(normalized) == 4 {
		normalized = string([]byte{'#', normalized[1], normalized[1], normalized[2], normalized[2], normalized[3], normalized[3]})
	}
	normalized = strings.ToUpper(normalized)
	if facilities.IsReservedRoomColor(normalized) {
		return "", facilities.ErrRoomColorReserved
	}
	return normalized, nil
}
