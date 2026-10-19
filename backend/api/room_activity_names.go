package api

import (
	"context"

	facilitiesNative "github.com/moto-nrw/project-phoenix/modules/facilities"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// roomActivityNames serves the activity group names the room occupancy and
// history projections label sessions with from the Timetable owner's groups.
type roomActivityNames struct{ groups timetable.Query }

func (n roomActivityNames) ListActivityGroupNames(ctx context.Context, ids []int64) (map[int64]facilitiesNative.ActivityGroupName, error) {
	groups, err := n.groups.ListGroups(ctx, timetable.GroupFilter{IDs: ids})
	if err != nil {
		return nil, err
	}
	names := make(map[int64]facilitiesNative.ActivityGroupName, len(groups))
	for _, group := range groups {
		name := facilitiesNative.ActivityGroupName{Name: group.Name}
		if group.Category != nil {
			name.Category = &group.Category.Name
		}
		names[group.ID] = name
	}
	return names, nil
}
