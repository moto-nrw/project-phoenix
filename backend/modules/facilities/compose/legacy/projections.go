package legacy

import (
	"context"
	"slices"
	"strings"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	facilitiesService "github.com/moto-nrw/project-phoenix/services/facilities"
)

func OccupancyProjection(
	groups activeModels.GroupRepository,
	activities activityGroupProjection,
	membership schoolmembership.Query,
	persons peopledirectory.Query,
) facilitiesService.OccupancyProjection {
	return func(ctx context.Context, rooms []facilitiesModule.Room) ([]facilitiesService.RoomWithOccupancy, error) {
		ids := make([]int64, len(rooms))
		for index := range rooms {
			ids[index] = rooms[index].ID
		}
		facts, err := groups.ListRoomOccupancy(ctx, ids)
		if err != nil {
			return nil, err
		}
		labels, err := roomActivityLabels(ctx, facts, activities)
		if err != nil {
			return nil, err
		}
		staffNames, err := roomSupervisorNames(ctx, facts, membership, persons)
		if err != nil {
			return nil, err
		}
		byRoom := make(map[int64]activeModels.RoomOccupancy, len(facts))
		for _, fact := range facts {
			byRoom[fact.RoomID] = fact
		}
		result := make([]facilitiesService.RoomWithOccupancy, 0, len(rooms))
		for index := range rooms {
			room := rooms[index]
			fact, occupied := byRoom[room.ID]
			label := labels[room.ID]
			result = append(result, facilitiesService.RoomWithOccupancy{
				Room: &room, IsOccupied: occupied, GroupName: label.GroupName,
				CategoryName: label.CategoryName, StudentCount: fact.StudentCount,
				SupervisorNames: staffNames[room.ID],
			})
		}
		return result, nil
	}
}

type activityGroupProjection interface {
	ListWithCategory(context.Context, *activityModels.GroupListQuery) ([]*activityModels.Group, error)
}

type activityLabel struct {
	GroupName    *string
	CategoryName *string
}

func roomActivityLabels(
	ctx context.Context,
	facts []activeModels.RoomOccupancy,
	activities activityGroupProjection,
) (map[int64]activityLabel, error) {
	groupIDs := make([]int64, 0)
	for _, fact := range facts {
		groupIDs = append(groupIDs, fact.ActivityGroupIDs...)
	}
	if len(groupIDs) == 0 {
		return map[int64]activityLabel{}, nil
	}
	groups, err := activities.ListWithCategory(ctx, &activityModels.GroupListQuery{IDs: groupIDs})
	if err != nil {
		return nil, err
	}
	groupsByID := make(map[int64]*activityModels.Group, len(groups))
	for _, group := range groups {
		groupsByID[group.ID] = group
	}
	result := make(map[int64]activityLabel, len(facts))
	for _, fact := range facts {
		result[fact.RoomID] = labelsForActivityGroups(fact.ActivityGroupIDs, groupsByID)
	}
	return result, nil
}

func labelsForActivityGroups(ids []int64, groups map[int64]*activityModels.Group) activityLabel {
	names := make([]string, 0, len(ids))
	categories := make([]string, 0, len(ids))
	for _, id := range ids {
		group := groups[id]
		if group == nil {
			continue
		}
		names = append(names, group.Name)
		if group.Category != nil {
			categories = append(categories, group.Category.Name)
		}
	}
	return activityLabel{GroupName: joinedSorted(names), CategoryName: joinedSorted(categories)}
}

func joinedSorted(values []string) *string {
	if len(values) == 0 {
		return nil
	}
	slices.Sort(values)
	values = slices.Compact(values)
	joined := strings.Join(values, ", ")
	return &joined
}

func roomSupervisorNames(
	ctx context.Context,
	facts []activeModels.RoomOccupancy,
	membership schoolmembership.Query,
	persons peopledirectory.Query,
) (map[int64]*string, error) {
	staffIDs := make([]int64, 0)
	for _, fact := range facts {
		staffIDs = append(staffIDs, fact.SupervisorStaffIDs...)
	}
	if len(staffIDs) == 0 {
		return map[int64]*string{}, nil
	}
	staff, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: staffIDs, IncludeDeleted: true})
	if err != nil {
		return nil, err
	}
	personIDsByStaff := make(map[int64]int64, len(staff))
	personIDs := make([]int64, 0, len(staff))
	for _, member := range staff {
		personIDsByStaff[member.ID] = member.PersonID
		personIDs = append(personIDs, member.PersonID)
	}
	people, err := persons.ListPersonsByID(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	nameByPerson := make(map[int64]string, len(people))
	for _, person := range people {
		nameByPerson[person.ID] = person.FirstName + " " + person.LastName
	}
	result := make(map[int64]*string, len(facts))
	for _, fact := range facts {
		names := make([]string, 0, len(fact.SupervisorStaffIDs))
		for _, staffID := range fact.SupervisorStaffIDs {
			if name := nameByPerson[personIDsByStaff[staffID]]; name != "" {
				names = append(names, name)
			}
		}
		slices.Sort(names)
		names = slices.Compact(names)
		if len(names) != 0 {
			joined := strings.Join(names, ", ")
			result[fact.RoomID] = &joined
		}
	}
	return result, nil
}

func HistoryProjection(groups activeModels.GroupRepository) facilitiesService.RoomHistoryProjection {
	return func(ctx context.Context, roomID int64, start, end time.Time, staffID *int64) ([]facilitiesService.RoomSessionEntry, error) {
		rows, err := groups.AggregateRoomSessions(ctx, roomID, start, end, staffID)
		if err != nil {
			return nil, err
		}
		result := make([]facilitiesService.RoomSessionEntry, 0, len(rows))
		for _, row := range rows {
			result = append(result, facilitiesService.RoomSessionEntry{
				SessionID: row.SessionID, StartedAt: row.StartedAt, EndedAt: row.EndedAt,
				DurationMinutes: row.DurationMinutes, ActivityName: row.ActivityName,
				SupervisorName: row.SupervisorName, StudentCount: row.StudentCount,
			})
		}
		return result, nil
	}
}
