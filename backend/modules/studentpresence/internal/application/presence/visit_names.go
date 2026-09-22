package presence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

func (s *service) todayVisitNames(ctx context.Context, studentIDs []int64) ([]visitGroupNames, error) {
	since := timezone.TodayDate().BerlinMidnight()
	locations, err := s.SchoolPresence.ListVisitLocations(ctx, studentpresence.VisitLocationFilter{VisitFilter: studentpresence.VisitFilter{
		StudentIDs: studentIDs, EnteredFrom: &since,
	}})
	if err != nil {
		return nil, err
	}
	roomIDs, groupIDs := visitLocationIDs(locations)
	roomNames, err := s.roomNamesByID(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	groupNames, err := s.activityNamesByID(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	result := make([]visitGroupNames, 0, len(locations))
	for _, location := range locations {
		row := visitGroupNames{StudentID: location.Visit.StudentID}
		if location.Group != nil {
			row.RoomName = roomNames[location.Group.RoomID]
			if location.Group.TemplateID != nil {
				row.ActivityGroupName = groupNames[*location.Group.TemplateID]
			}
		}
		result = append(result, row)
	}
	return result, nil
}

// visitLocationIDs lists the rooms and activities of the visits' sessions.
func visitLocationIDs(locations []studentpresence.VisitLocation) (roomIDs, groupIDs []int64) {
	roomIDs = make([]int64, 0, len(locations))
	groupIDs = make([]int64, 0, len(locations))
	for _, location := range locations {
		if location.Group == nil {
			continue
		}
		roomIDs = append(roomIDs, location.Group.RoomID)
		if location.Group.TemplateID != nil {
			groupIDs = append(groupIDs, *location.Group.TemplateID)
		}
	}
	return roomIDs, groupIDs
}

func (s *service) roomNamesByID(ctx context.Context, roomIDs []int64) (map[int64]string, error) {
	roomNames := make(map[int64]string)
	if len(roomIDs) == 0 {
		return roomNames, nil
	}
	rooms, err := s.RoomRepo.FindByIDs(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	for _, room := range rooms {
		roomNames[room.ID] = room.Name
	}
	return roomNames, nil
}

func (s *service) activityNamesByID(ctx context.Context, groupIDs []int64) (map[int64]string, error) {
	groupNames := make(map[int64]string)
	if len(groupIDs) == 0 {
		return groupNames, nil
	}
	groups, err := s.ActivityGroupRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		groupNames[group.ID] = group.Name
	}
	return groupNames, nil
}

// visitGroupNames holds the activity and room names from a visit for indicator matching.
type visitGroupNames struct {
	StudentID         int64
	ActivityGroupName string
	RoomName          string
}
