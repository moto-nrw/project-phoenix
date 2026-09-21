package presence

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// ======== Unclaimed Groups Management (Deviceless Claiming) ========

// GetUnclaimedActiveGroups returns all active groups that have no supervisors
// This is used for deviceless rooms like Schulhof where teachers claim supervision via frontend
func (s *service) GetUnclaimedActiveGroups(ctx context.Context) ([]*active.Group, error) {
	rows, err := s.SchoolPresence.UnclaimedGroups(ctx, s.todayDate().String())
	if err != nil {
		return nil, &ActiveError{Op: "GetUnclaimedActiveGroups", Err: err}
	}
	groups := make([]*active.Group, 0, len(rows))
	if len(rows) == 0 {
		return groups, nil
	}
	roomIDs, templateIDs := make([]int64, 0, len(rows)), make([]int64, 0, len(rows))
	for _, row := range rows {
		roomIDs = append(roomIDs, row.RoomID)
		if row.GroupID != nil {
			templateIDs = append(templateIDs, *row.GroupID)
		}
	}
	rooms, err := s.RoomRepo.FindByIDs(ctx, roomIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetUnclaimedActiveGroups", Err: err}
	}
	templates, err := s.ActivityGroupRepo.FindByIDs(ctx, templateIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetUnclaimedActiveGroups", Err: err}
	}
	roomsByID := make(map[int64]*active.SessionRoom, len(rooms))
	for _, room := range rooms {
		roomsByID[room.ID] = room
	}
	templatesByID := make(map[int64]*active.SessionActivity, len(templates))
	for _, template := range templates {
		// This endpoint historically includes the template without its category relation.
		copy := *template
		copy.Category = nil
		templatesByID[copy.ID] = &copy
	}
	for _, row := range rows {
		group := &active.Group{Model: base.Model{ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, StartTime: row.StartTime, EndTime: row.EndTime, LastActivity: row.LastActivity, TimeoutMinutes: row.TimeoutMinutes, GroupID: row.GroupID, DeviceID: row.DeviceID, RoomID: row.RoomID}
		group.SetTenantID(row.TenantID)
		group.Room = roomsByID[row.RoomID]
		if group.Room == nil || group.Room.Name != "Schulhof" {
			continue
		}
		if row.GroupID != nil {
			group.ActualGroup = templatesByID[*row.GroupID]
		}
		groups = append(groups, group)
	}
	return groups, nil
}

// ClaimActiveGroup allows a staff member to claim supervision of an active group
// This is primarily used for deviceless rooms like Schulhof
func (s *service) ClaimActiveGroup(ctx context.Context, groupID, staffID int64, role string) (*active.GroupSupervisor, error) {
	if role == "" {
		role = "supervisor"
	}
	var result *active.GroupSupervisor
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		if err := s.lockStaffForSupervision(txCtx, staffID); err != nil {
			return err
		}
		row, err := s.SchoolPresence.ClaimGroup(txCtx, studentpresence.GroupClaim{GroupID: groupID, StaffID: staffID, Role: role, Date: s.todayDate().String()})
		switch {
		case errors.Is(err, studentpresence.ErrAlreadySupervising):
			return ErrStaffAlreadySupervising
		case errors.Is(err, studentpresence.ErrGroupNotFound), errors.Is(err, studentpresence.ErrGroupEnded):
			return err
		case err != nil:
			return ErrDatabaseOperation
		}
		date, err := timezone.ParseDate(row.StartDate)
		if err != nil {
			return err
		}
		result = &active.GroupSupervisor{Model: base.Model{ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, GroupID: row.GroupID, StaffID: row.StaffID, Role: row.Role, StartDate: date}
		result.SetTenantID(row.TenantID)
		source := stampSourceApp
		if s.attendancePrincipal(txCtx).IsIoT {
			source = stampSourceNFC
		}
		s.ensureStaffPresence(txCtx, staffID, source)
		return nil
	})
	if err != nil {
		return nil, &ActiveError{Op: "ClaimActiveGroup", Err: err}
	}
	return result, nil
}
