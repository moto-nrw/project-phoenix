package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
)

// CallerActivityGroups serves the Identity & Access caller context and the
// rows it renders the planned activity groups a staff member supervises,
// read from the owner's group and planned supervisor capability (#3501).
type CallerActivityGroups struct {
	timetable timetable.Query
}

// NewCallerActivityGroups binds the reads to the owner's capability.
func NewCallerActivityGroups(owner timetable.Query) CallerActivityGroups {
	return CallerActivityGroups{timetable: owner}
}

// SupervisedActivityGroupIDs reads the activity groups a staff member
// supervises as planned.
func (g CallerActivityGroups) SupervisedActivityGroupIDs(ctx context.Context, staffID int64) ([]int64, error) {
	supervisors, err := g.timetable.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{StaffID: &staffID})
	if err != nil {
		return nil, legacyDatabaseError("find by staff supervisor", err)
	}
	groupIDs := uniqueSupervisorGroupIDs(supervisors)
	if len(groupIDs) == 0 {
		return []int64{}, nil
	}
	groups, err := g.timetable.ListGroups(ctx, timetable.GroupFilter{IDs: groupIDs})
	if err != nil {
		return nil, legacyDatabaseError("find by staff supervisor", err)
	}
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	return ids, nil
}

// ActivityGroupRows returns the planned activity groups the
// /api/me/groups/activity route renders, in the order of ids.
func (g CallerActivityGroups) ActivityGroupRows(ctx context.Context, ids []int64) (any, error) {
	result := make([]*activityContract.Group, 0, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := legacyGroupsByID(ctx, g.timetable, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*activityContract.Group, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	for _, id := range ids {
		if row := byID[id]; row != nil {
			result = append(result, row)
		}
	}
	return result, nil
}
