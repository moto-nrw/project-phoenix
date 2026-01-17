package education

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	"github.com/uptrace/bun"
)

// GetGroup retrieves a group by its ID
func (s *service) GetGroup(ctx context.Context, id int64) (*education.Group, error) {
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &EducationError{Op: "GetGroup", Err: ErrGroupNotFound}
	}
	return group, nil
}

// GetGroupsByIDs retrieves multiple groups by their IDs in a single query
func (s *service) GetGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*education.Group, error) {
	if len(ids) == 0 {
		return make(map[int64]*education.Group), nil
	}

	groups, err := s.groupRepo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, &EducationError{Op: "GetGroupsByIDs", Err: err}
	}

	return groups, nil
}

// CreateGroup creates a new education group
func (s *service) CreateGroup(ctx context.Context, group *education.Group) error {
	// Validate group data
	if err := group.Validate(); err != nil {
		return &EducationError{Op: "CreateGroup", Err: err}
	}

	// Check if group with same name already exists
	existingGroup, err := s.groupRepo.FindByName(ctx, group.Name)
	if err == nil && existingGroup != nil && existingGroup.ID > 0 {
		return &EducationError{Op: "CreateGroup", Err: ErrDuplicateGroup}
	}

	// If room ID is specified, verify the room exists
	if group.RoomID != nil && *group.RoomID > 0 {
		room, err := s.roomRepo.FindByID(ctx, *group.RoomID)
		if err != nil {
			return &EducationError{Op: "CreateGroup", Err: ErrRoomNotFound}
		}
		group.Room = room
	}

	// Create the group
	if err := s.groupRepo.Create(ctx, group); err != nil {
		return &EducationError{Op: "CreateGroup", Err: err}
	}

	return nil
}

// UpdateGroup updates an existing education group
func (s *service) UpdateGroup(ctx context.Context, group *education.Group) error {
	if err := group.Validate(); err != nil {
		return &EducationError{Op: "UpdateGroup", Err: err}
	}

	existingGroup, err := s.groupRepo.FindByID(ctx, group.ID)
	if err != nil {
		return &EducationError{Op: "UpdateGroup", Err: ErrGroupNotFound}
	}

	if err := s.checkGroupNameUnique(ctx, existingGroup, group); err != nil {
		return err
	}

	if err := s.validateAndSetRoom(ctx, existingGroup, group); err != nil {
		return err
	}

	if err := s.groupRepo.Update(ctx, group); err != nil {
		return &EducationError{Op: "UpdateGroup", Err: err}
	}

	return nil
}

// checkGroupNameUnique checks if name changed and validates no duplicates
func (s *service) checkGroupNameUnique(ctx context.Context, existing, updated *education.Group) error {
	if existing.Name == updated.Name {
		return nil
	}

	nameGroup, err := s.groupRepo.FindByName(ctx, updated.Name)
	if err == nil && nameGroup != nil && nameGroup.ID != updated.ID {
		return &EducationError{Op: "UpdateGroup", Err: ErrDuplicateGroup}
	}

	return nil
}

// validateAndSetRoom validates room change and sets room reference
func (s *service) validateAndSetRoom(ctx context.Context, existing, updated *education.Group) error {
	if !roomIDHasChanged(existing.RoomID, updated.RoomID) {
		return nil
	}

	if updated.RoomID != nil && *updated.RoomID > 0 {
		room, err := s.roomRepo.FindByID(ctx, *updated.RoomID)
		if err != nil {
			return &EducationError{Op: "UpdateGroup", Err: ErrRoomNotFound}
		}
		updated.Room = room
	} else {
		updated.Room = nil
	}

	return nil
}

// roomIDHasChanged checks if room ID has changed (handles nil comparisons)
func roomIDHasChanged(oldRoomID, newRoomID *int64) bool {
	if oldRoomID == nil && newRoomID != nil {
		return true
	}
	if oldRoomID != nil && newRoomID == nil {
		return true
	}
	if oldRoomID != nil && newRoomID != nil && *oldRoomID != *newRoomID {
		return true
	}
	return false
}

// DeleteGroup deletes an education group by ID
func (s *service) DeleteGroup(ctx context.Context, id int64) error {
	if _, err := s.groupRepo.FindByID(ctx, id); err != nil {
		return &EducationError{Op: "DeleteGroup", Err: ErrGroupNotFound}
	}

	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(Service)

		if err := deleteGroupTeacherRelations(ctx, txService, id); err != nil {
			return err
		}

		if err := deleteGroupSubstitutions(ctx, txService, id); err != nil {
			return err
		}

		// Use transaction-bound repo for delete to maintain consistency
		return txService.(*service).groupRepo.Delete(ctx, id)
	})

	if err != nil {
		return &EducationError{Op: "DeleteGroup", Err: err}
	}

	return nil
}

// deleteGroupTeacherRelations deletes all teacher relationships for a group
func deleteGroupTeacherRelations(ctx context.Context, service Service, groupID int64) error {
	groupTeachers, err := service.GetGroupTeachers(ctx, groupID)
	if err != nil || len(groupTeachers) == 0 {
		return nil
	}

	for _, teacher := range groupTeachers {
		if err := service.RemoveTeacherFromGroup(ctx, groupID, teacher.ID); err != nil {
			return err
		}
	}

	return nil
}

// deleteGroupSubstitutions deletes all substitutions for a group
func deleteGroupSubstitutions(ctx context.Context, service Service, groupID int64) error {
	substitutions, err := service.GetActiveGroupSubstitutions(ctx, groupID, time.Now())
	if err != nil || len(substitutions) == 0 {
		return nil
	}

	for _, sub := range substitutions {
		if err := service.DeleteSubstitution(ctx, sub.ID); err != nil {
			return err
		}
	}

	return nil
}

// ListGroups retrieves groups with optional filtering
func (s *service) ListGroups(ctx context.Context, options *base.QueryOptions) ([]*education.Group, error) {
	// Now we can directly use the modern ListWithOptions method
	groups, err := s.groupRepo.ListWithOptions(ctx, options)
	if err != nil {
		return nil, &EducationError{Op: "ListGroups", Err: err}
	}
	return groups, nil
}

// FindGroupByName finds a group by its name
func (s *service) FindGroupByName(ctx context.Context, name string) (*education.Group, error) {
	group, err := s.groupRepo.FindByName(ctx, name)
	if err != nil {
		return nil, &EducationError{Op: "FindGroupByName", Err: ErrGroupNotFound}
	}
	return group, nil
}

// FindGroupsByRoom finds all groups assigned to a specific room
func (s *service) FindGroupsByRoom(ctx context.Context, roomID int64) ([]*education.Group, error) {
	// Verify room exists
	_, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil {
		return nil, &EducationError{Op: "FindGroupsByRoom", Err: ErrRoomNotFound}
	}

	groups, err := s.groupRepo.FindByRoom(ctx, roomID)
	if err != nil {
		return nil, &EducationError{Op: "FindGroupsByRoom", Err: err}
	}
	return groups, nil
}

// FindGroupWithRoom retrieves a group with its associated room
func (s *service) FindGroupWithRoom(ctx context.Context, groupID int64) (*education.Group, error) {
	group, err := s.groupRepo.FindWithRoom(ctx, groupID)
	if err != nil {
		return nil, &EducationError{Op: "FindGroupWithRoom", Err: ErrGroupNotFound}
	}
	return group, nil
}

// AssignRoomToGroup assigns a room to a group
func (s *service) AssignRoomToGroup(ctx context.Context, groupID, roomID int64) error {
	// Verify group exists
	group, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return &EducationError{Op: "AssignRoomToGroup", Err: ErrGroupNotFound}
	}

	// Verify room exists
	room, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil {
		return &EducationError{Op: "AssignRoomToGroup", Err: ErrRoomNotFound}
	}

	// Update group's room
	group.SetRoom(room)
	if err := s.groupRepo.Update(ctx, group); err != nil {
		return &EducationError{Op: "AssignRoomToGroup", Err: err}
	}

	return nil
}

// RemoveRoomFromGroup removes a room assignment from a group
func (s *service) RemoveRoomFromGroup(ctx context.Context, groupID int64) error {
	// Verify group exists
	group, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return &EducationError{Op: "RemoveRoomFromGroup", Err: ErrGroupNotFound}
	}

	// Remove room assignment
	group.SetRoom(nil)
	if err := s.groupRepo.Update(ctx, group); err != nil {
		return &EducationError{Op: "RemoveRoomFromGroup", Err: err}
	}

	return nil
}
