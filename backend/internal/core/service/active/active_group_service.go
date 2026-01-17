package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

// Active Group operations
func (s *service) GetActiveGroup(ctx context.Context, id int64) (*active.Group, error) {
	group, err := s.groupReadRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroup", Err: ErrActiveGroupNotFound}
	}

	// Ensure we always have room metadata so downstream callers
	// (location resolver, SSE payloads) can render friendly labels.
	if group != nil && group.Room == nil && group.RoomID > 0 {
		if room, roomErr := s.roomRepo.FindByID(ctx, group.RoomID); roomErr == nil {
			group.Room = room
		}
	}

	return group, nil
}

func (s *service) GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*active.Group, error) {
	if len(groupIDs) == 0 {
		return map[int64]*active.Group{}, nil
	}

	groups, err := s.groupReadRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupsByIDs", Err: ErrDatabaseOperation}
	}

	if groups == nil {
		groups = make(map[int64]*active.Group)
	}

	return groups, nil
}

func (s *service) CreateActiveGroup(ctx context.Context, group *active.Group) error {
	if group == nil || group.Validate() != nil {
		return &ActiveError{Op: "CreateActiveGroup", Err: ErrInvalidData}
	}

	// Check for room conflicts if room is assigned
	if group.RoomID > 0 {
		hasConflict, _, err := s.groupReadRepo.CheckRoomConflict(ctx, group.RoomID, 0)
		if err != nil {
			return &ActiveError{Op: "CreateActiveGroup", Err: fmt.Errorf("check room conflict: %w", err)}
		}
		if hasConflict {
			return &ActiveError{Op: "CreateActiveGroup", Err: ErrRoomConflict}
		}
	}

	if err := s.groupWriteRepo.Create(ctx, group); err != nil {
		return &ActiveError{Op: "CreateActiveGroup", Err: fmt.Errorf("create failed: %w", err)}
	}

	return nil
}

func (s *service) UpdateActiveGroup(ctx context.Context, group *active.Group) error {
	if group == nil || group.Validate() != nil {
		return &ActiveError{Op: "UpdateActiveGroup", Err: ErrInvalidData}
	}

	// Check for room conflicts if room is assigned (exclude current group)
	if group.RoomID > 0 {
		hasConflict, _, err := s.groupReadRepo.CheckRoomConflict(ctx, group.RoomID, group.ID)
		if err != nil {
			return &ActiveError{Op: "UpdateActiveGroup", Err: fmt.Errorf("check room conflict: %w", err)}
		}
		if hasConflict {
			return &ActiveError{Op: "UpdateActiveGroup", Err: ErrRoomConflict}
		}
	}

	if err := s.groupWriteRepo.Update(ctx, group); err != nil {
		return &ActiveError{Op: "UpdateActiveGroup", Err: fmt.Errorf("update failed: %w", err)}
	}

	return nil
}

func (s *service) DeleteActiveGroup(ctx context.Context, id int64) error {
	// Check if there are any active visits for this group
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, id)
	if err != nil {
		return &ActiveError{Op: "DeleteActiveGroup", Err: fmt.Errorf("find visits: %w", err)}
	}

	// Check if any of the visits are still active
	for _, visit := range visits {
		if visit.IsActive() {
			return &ActiveError{Op: "DeleteActiveGroup", Err: ErrCannotDeleteActiveGroup}
		}
	}

	// Delete the active group
	_, err = s.groupReadRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ActiveError{Op: "DeleteActiveGroup", Err: ErrActiveGroupNotFound}
		}
		return &ActiveError{Op: "DeleteActiveGroup", Err: fmt.Errorf("find group: %w", err)}
	}

	if err := s.groupWriteRepo.Delete(ctx, id); err != nil {
		return &ActiveError{Op: "DeleteActiveGroup", Err: fmt.Errorf("delete failed: %w", err)}
	}

	return nil
}

func (s *service) ListActiveGroups(ctx context.Context, options *base.QueryOptions) ([]*active.Group, error) {
	groups, err := s.groupReadRepo.List(ctx, options)
	if err != nil {
		return nil, &ActiveError{Op: "ListActiveGroups", Err: fmt.Errorf("list failed: %w", err)}
	}
	return groups, nil
}

func (s *service) FindActiveGroupsByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error) {
	groups, err := s.groupReadRepo.FindActiveByRoomID(ctx, roomID)
	if err != nil {
		return nil, &ActiveError{Op: "FindActiveGroupsByRoomID", Err: fmt.Errorf("find by room: %w", err)}
	}
	return groups, nil
}

func (s *service) FindActiveGroupsByGroupID(ctx context.Context, groupID int64) ([]*active.Group, error) {
	groups, err := s.groupReadRepo.FindActiveByGroupID(ctx, groupID)
	if err != nil {
		return nil, &ActiveError{Op: "FindActiveGroupsByGroupID", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) FindActiveGroupsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Group, error) {
	if start.After(end) {
		return nil, &ActiveError{Op: "FindActiveGroupsByTimeRange", Err: ErrInvalidTimeRange}
	}

	groups, err := s.groupReadRepo.FindByTimeRange(ctx, start, end)
	if err != nil {
		return nil, &ActiveError{Op: "FindActiveGroupsByTimeRange", Err: ErrDatabaseOperation}
	}
	return groups, nil
}

func (s *service) EndActiveGroupSession(ctx context.Context, id int64) error {
	// Delegate to EndActivitySession which properly ends visits and broadcasts SSE
	if err := s.EndActivitySession(ctx, id); err != nil {
		// Wrap the error with our operation name for clarity
		if activeErr, ok := err.(*ActiveError); ok {
			return &ActiveError{Op: "EndActiveGroupSession", Err: activeErr.Err}
		}
		return &ActiveError{Op: "EndActiveGroupSession", Err: err}
	}
	return nil
}

func (s *service) GetActiveGroupWithVisits(ctx context.Context, id int64) (*active.Group, error) {
	// Get the active group
	group, err := s.groupReadRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithVisits", Err: ErrActiveGroupNotFound}
	}

	// Get visits for this group
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithVisits", Err: ErrDatabaseOperation}
	}

	group.Visits = visits
	return group, nil
}

func (s *service) GetActiveGroupWithSupervisors(ctx context.Context, id int64) (*active.Group, error) {
	// Get the active group
	group, err := s.groupReadRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithSupervisors", Err: ErrActiveGroupNotFound}
	}

	// Get supervisors for this group (only active ones)
	supervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, id, true)
	if err != nil {
		return nil, &ActiveError{Op: "GetActiveGroupWithSupervisors", Err: ErrDatabaseOperation}
	}

	group.Supervisors = supervisors
	return group, nil
}
