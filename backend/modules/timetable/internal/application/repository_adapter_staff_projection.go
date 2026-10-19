package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
)

// staffActivityGroupRepository attaches the staff member to the supervisors
// a group is loaded with. The replaced lookup was a soft-delete-aware model
// select, so an offboarded supervisor keeps a nil Staff here.
type staffActivityGroupRepository struct {
	ports.ActivityGroupRecords
	membership func(context.Context, []int64, bool) (map[int64]int64, error)
}

func (r staffActivityGroupRepository) FindWithSupervisors(ctx context.Context, groupID int64) (*activityContract.Group, []*activityContract.SupervisorPlanned, error) {
	group, supervisors, err := r.ActivityGroupRecords.FindWithSupervisors(ctx, groupID)
	if err != nil {
		return nil, nil, err
	}
	if err := attachPlannedSupervisorStaff(ctx, r.membership, supervisors, false); err != nil {
		return nil, nil, err
	}
	return group, supervisors, nil
}

// staffActivityGroupTargetRepository keeps the optional target seam the
// timetable services type-assert on reachable through the staff wrapper.
type staffActivityGroupTargetRepository struct {
	staffActivityGroupRepository
	ports.ActivityGroupTargets
}

var _ activityGroupTargets = staffActivityGroupTargetRepository{}

func newStaffActivityGroupRepository(inner ports.ActivityGroupRecords, membership func(context.Context, []int64, bool) (map[int64]int64, error)) ports.ActivityGroupRecords {
	wrapped := staffActivityGroupRepository{ActivityGroupRecords: inner, membership: membership}
	if targets, ok := inner.(ports.ActivityGroupTargets); ok {
		return staffActivityGroupTargetRepository{staffActivityGroupRepository: wrapped, ActivityGroupTargets: targets}
	}
	return wrapped
}

// staffSupervisorPlannedRepository attaches the staff member to planned
// supervisor rows. The replaced LEFT JOIN carried no soft-delete filter, so
// an offboarded supervisor still resolves.
type staffSupervisorPlannedRepository struct {
	ports.PlannedSupervisorRecords
	membership func(context.Context, []int64, bool) (map[int64]int64, error)
}

func (r staffSupervisorPlannedRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activityContract.SupervisorPlanned, error) {
	rows, err := r.PlannedSupervisorRecords.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return rows, attachPlannedSupervisorStaff(ctx, r.membership, rows, true)
}

func (r staffSupervisorPlannedRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activityContract.SupervisorPlanned, error) {
	rows, err := r.PlannedSupervisorRecords.FindByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	return rows, attachPlannedSupervisorStaff(ctx, r.membership, rows, true)
}

func attachPlannedSupervisorStaff(ctx context.Context, staffPersons func(context.Context, []int64, bool) (map[int64]int64, error), rows []*activityContract.SupervisorPlanned, includeDeleted bool) error {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			ids = append(ids, row.StaffID)
		}
	}
	members, err := staffPersons(ctx, ids, includeDeleted)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		if personID, found := members[row.StaffID]; found {
			row.StaffPersonID = personID
		}
	}
	return nil
}
