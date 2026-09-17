package timetableplanning

import (
	"context"
	"errors"
	"slices"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// OperationSessionBlock is the timetable block running behind one live
// session, seen by the caller (#3281). StartTime/EndTime are the plan window.
type OperationSessionBlock struct {
	ActiveGroupID int64
	InstanceID    int64
	Title         string
	StartTime     string
	EndTime       string
	// IsAssigned reports a plan entry of the caller that is not marked absent.
	IsAssigned bool
	// CanOperate is requireCanOperate's verdict for the caller.
	CanOperate bool
}

// SessionBlocks resolves the running blocks behind the given live sessions of
// the day in bulk. supervisorStaffIDs maps each session to its current
// supervisors as the caller already read them; the rule that turns plan and
// supervision into CanOperate is the one requireCanOperate applies to a
// single block. Sessions without a running block are left out. The read costs
// a fixed number of statements, whatever the number of sessions.
func (s *timetableOperationsService) SessionBlocks(ctx context.Context, accountID int64, isAdmin bool, date timezone.Date, supervisorStaffIDs map[int64][]int64) ([]OperationSessionBlock, error) {
	result := []OperationSessionBlock{}
	if len(supervisorStaffIDs) == 0 {
		return result, nil
	}
	instances, err := s.deps.InstanceRepo.FindByTenantAndDate(ctx, scheduleModel.Date(date))
	if err != nil {
		return nil, err
	}
	running := make([]*scheduleModel.ActivityInstance, 0, len(supervisorStaffIDs))
	for _, inst := range instances {
		if inst.Status != scheduleModel.InstanceStatusActive || inst.ActiveGroupID == nil {
			continue
		}
		if _, asked := supervisorStaffIDs[*inst.ActiveGroupID]; asked {
			running = append(running, inst)
		}
	}
	if len(running) == 0 {
		return result, nil
	}
	staffRows, err := s.deps.InstanceStaffRepo.FindByInstanceIDs(ctx, activityInstanceIDs(running))
	if err != nil {
		return nil, err
	}
	staffByInstance := indexInstanceStaffRows(staffRows)

	// An account requireCanOperate cannot resolve at all operates no block,
	// admins included; it has no staff profile either.
	staffID, hasStaff, err := s.resolveStaffID(ctx, accountID)
	if err != nil && !errors.Is(err, ErrTimetableOperationForbidden) {
		return nil, err
	}
	adminActions := err == nil && s.hasAdministrativeActionAccess(ctx, isAdmin)

	for _, inst := range running {
		activeGroupID := *inst.ActiveGroupID
		rows := staffByInstance[inst.ID]
		block := OperationSessionBlock{
			ActiveGroupID: activeGroupID,
			InstanceID:    inst.ID,
			Title:         inst.Title,
			StartTime:     inst.StartTime.Format("15:04"),
			EndTime:       inst.EndTime.Format("15:04"),
			IsAssigned:    hasStaff && staffAssigned(rows, staffID),
			CanOperate:    adminActions,
		}
		if !adminActions && hasStaff {
			block.CanOperate, err = s.operatesLoadedBlock(ctx, inst, rows, staffID, func() (bool, error) {
				return slices.Contains(supervisorStaffIDs[activeGroupID], staffID), nil
			})
			if err != nil {
				return nil, err
			}
		}
		result = append(result, block)
	}
	return result, nil
}
