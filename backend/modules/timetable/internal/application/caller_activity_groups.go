package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
)

// CallerActivityGroups serves the Identity & Access caller context and the
// rows it renders the planned activity groups a staff member supervises,
// read from the retained activity group rows (#3501).
type CallerActivityGroups struct {
	groups ports.ActivityGroupRecords
}

// NewCallerActivityGroups binds the reads to the retained activity groups.
func NewCallerActivityGroups(groups ports.ActivityGroupRecords) CallerActivityGroups {
	return CallerActivityGroups{groups: groups}
}

// SupervisedActivityGroupIDs reads the activity groups a staff member
// supervises as planned.
func (g CallerActivityGroups) SupervisedActivityGroupIDs(ctx context.Context, staffID int64) ([]int64, error) {
	rows, err := g.groups.FindByStaffSupervisor(ctx, staffID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
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
	rows, err := g.groups.FindByIDs(ctx, ids)
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
