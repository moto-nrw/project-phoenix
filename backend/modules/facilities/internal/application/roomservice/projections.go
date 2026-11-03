package facilities

import (
	"context"
	"slices"
	"strings"
	"time"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

func NewOccupancyProjection(
	groups facilitiesModule.RoomOccupancyQuery,
	activities facilitiesModule.ActivityGroupNames,
	membership schoolmembership.Query,
	persons peopledirectory.Query,
) OccupancyProjection {
	return func(ctx context.Context, rooms []facilitiesModule.Room) ([]RoomWithOccupancy, error) {
		return projectRoomOccupancy(ctx, rooms, groups, activities, membership, persons)
	}
}

func projectRoomOccupancy(
	ctx context.Context,
	rooms []facilitiesModule.Room,
	groups facilitiesModule.RoomOccupancyQuery,
	activities facilitiesModule.ActivityGroupNames,
	membership schoolmembership.Query,
	persons peopledirectory.Query,
) ([]RoomWithOccupancy, error) {
	facts, err := groups.ListRoomOccupancy(ctx, roomIDs(rooms))
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
	return occupancyViews(rooms, facts, labels, staffNames), nil
}

func roomIDs(rooms []facilitiesModule.Room) []int64 {
	ids := make([]int64, len(rooms))
	for index := range rooms {
		ids[index] = rooms[index].ID
	}
	return ids
}

func occupancyViews(
	rooms []facilitiesModule.Room,
	facts []facilitiesModule.RoomOccupancyFact,
	labels map[int64]activityLabel,
	staffNames map[int64]*string,
) []RoomWithOccupancy {
	byRoom := make(map[int64]facilitiesModule.RoomOccupancyFact, len(facts))
	for _, fact := range facts {
		byRoom[fact.RoomID] = fact
	}
	result := make([]RoomWithOccupancy, 0, len(rooms))
	for index := range rooms {
		room := rooms[index]
		fact, occupied := byRoom[room.ID]
		label := labels[room.ID]
		result = append(result, RoomWithOccupancy{
			Room: &room, IsOccupied: occupied, GroupName: label.GroupName,
			CategoryName: label.CategoryName, StudentCount: fact.StudentCount,
			SupervisorNames: staffNames[room.ID],
		})
	}
	return result
}

type activityLabel struct {
	GroupName    *string
	CategoryName *string
}

func roomActivityLabels(
	ctx context.Context,
	facts []facilitiesModule.RoomOccupancyFact,
	activities facilitiesModule.ActivityGroupNames,
) (map[int64]activityLabel, error) {
	groupIDs := make([]int64, 0)
	for _, fact := range facts {
		groupIDs = append(groupIDs, fact.ActivityGroupIDs...)
	}
	if len(groupIDs) == 0 {
		return map[int64]activityLabel{}, nil
	}
	groups, err := activities.ListActivityGroupNames(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]activityLabel, len(facts))
	for _, fact := range facts {
		result[fact.RoomID] = labelsForActivityGroups(fact.ActivityGroupIDs, groups)
	}
	return result, nil
}

func labelsForActivityGroups(ids []int64, groups map[int64]facilitiesModule.ActivityGroupName) activityLabel {
	names := make([]string, 0, len(ids))
	categories := make([]string, 0, len(ids))
	for _, id := range ids {
		group, ok := groups[id]
		if !ok {
			continue
		}
		names = append(names, group.Name)
		if group.Category != nil {
			categories = append(categories, *group.Category)
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
	facts []facilitiesModule.RoomOccupancyFact,
	membership schoolmembership.Query,
	persons peopledirectory.Query,
) (map[int64]*string, error) {
	staffIDs := supervisorStaffIDs(facts)
	if len(staffIDs) == 0 {
		return map[int64]*string{}, nil
	}
	staff, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: staffIDs, IncludeDeleted: true})
	if err != nil {
		return nil, err
	}
	personIDsByStaff, personIDs := staffPersonIDs(staff)
	people, err := persons.ListPersonsByID(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	return supervisorNamesByRoom(facts, personIDsByStaff, personNames(people)), nil
}

func supervisorStaffIDs(facts []facilitiesModule.RoomOccupancyFact) []int64 {
	staffIDs := make([]int64, 0)
	for _, fact := range facts {
		staffIDs = append(staffIDs, fact.SupervisorStaffIDs...)
	}
	return staffIDs
}

func staffPersonIDs(staff []schoolmembership.Staff) (map[int64]int64, []int64) {
	byStaff := make(map[int64]int64, len(staff))
	personIDs := make([]int64, 0, len(staff))
	for _, member := range staff {
		byStaff[member.ID] = member.PersonID
		personIDs = append(personIDs, member.PersonID)
	}
	return byStaff, personIDs
}

func personNames(people []peopledirectory.Person) map[int64]string {
	names := make(map[int64]string, len(people))
	for _, person := range people {
		names[person.ID] = person.FirstName + " " + person.LastName
	}
	return names
}

func supervisorNamesByRoom(
	facts []facilitiesModule.RoomOccupancyFact,
	personIDsByStaff map[int64]int64,
	nameByPerson map[int64]string,
) map[int64]*string {
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
	return result
}

func NewHistoryProjection(groups facilitiesModule.RoomHistoryQuery, activities facilitiesModule.ActivityGroupNames, membership schoolmembership.Query, persons peopledirectory.Query) RoomHistoryProjection {
	return func(ctx context.Context, roomID int64, start, end time.Time, staffID *int64) ([]facilitiesModule.RoomSessionEntry, error) {
		rows, err := groups.ListRoomSessionHistory(ctx, roomID, start, end, staffID)
		if err != nil {
			return nil, err
		}
		activityNames, supervisorNames, err := historyNames(ctx, rows, activities, membership, persons)
		if err != nil {
			return nil, err
		}
		result := make([]facilitiesModule.RoomSessionEntry, 0, len(rows))
		for _, row := range rows {
			result = append(result, facilitiesModule.RoomSessionEntry{
				SessionID: row.SessionID, StartedAt: row.StartedAt, EndedAt: row.EndedAt,
				DurationMinutes: row.DurationMinutes, ActivityName: activityNames[row.SessionID],
				SupervisorName: supervisorNames[row.SessionID], StudentCount: row.StudentCount,
			})
		}
		return result, nil
	}
}

func historyNames(ctx context.Context, rows []facilitiesModule.RoomSessionFact, activities facilitiesModule.ActivityGroupNames, membership schoolmembership.Query, persons peopledirectory.Query) (map[int64]string, map[int64]string, error) {
	activityIDs, staffIDs := []int64{}, []int64{}
	for _, row := range rows {
		if row.ActivityGroupID != nil {
			activityIDs = append(activityIDs, *row.ActivityGroupID)
		}
		staffIDs = append(staffIDs, row.SupervisorStaffIDs...)
	}
	activityNames, supervisorNames := map[int64]string{}, map[int64]string{}
	if len(activityIDs) > 0 {
		groups, err := activities.ListActivityGroupNames(ctx, activityIDs)
		if err != nil {
			return nil, nil, err
		}
		for _, row := range rows {
			if row.ActivityGroupID != nil {
				activityNames[row.SessionID] = groups[*row.ActivityGroupID].Name
			}
		}
	}
	if len(staffIDs) == 0 {
		return activityNames, supervisorNames, nil
	}
	supervisorNames, err := historySupervisorNames(ctx, rows, staffIDs, membership, persons)
	if err != nil {
		return nil, nil, err
	}
	return activityNames, supervisorNames, nil
}

func historySupervisorNames(ctx context.Context, rows []facilitiesModule.RoomSessionFact, staffIDs []int64, membership schoolmembership.Query, persons peopledirectory.Query) (map[int64]string, error) {
	supervisorNames := map[int64]string{}
	staff, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: staffIDs, IncludeDeleted: true})
	if err != nil {
		return nil, err
	}
	byStaff, personIDs := staffPersonIDs(staff)
	people, err := persons.ListPersonsByID(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	names := personNames(people)
	for _, row := range rows {
		supervisorNames[row.SessionID] = historySupervisorLabel(row.SupervisorStaffIDs, byStaff, names)
	}

	return supervisorNames, nil
}

func historySupervisorLabel(staffIDs []int64, byStaff map[int64]int64, names map[int64]string) string {
	values := []string{}
	for _, id := range staffIDs {
		if personID, ok := byStaff[id]; ok {
			if name, ok := names[personID]; ok {
				values = append(values, strings.TrimSpace(name))
			}
		}
	}
	slices.Sort(values)
	return strings.Join(slices.Compact(values), ", ")
}
