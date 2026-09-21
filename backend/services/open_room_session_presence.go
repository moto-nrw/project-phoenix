package services

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type openRoomActivityQuery interface {
	ListGroups(context.Context, timetable.GroupFilter) ([]timetable.Group, error)
}
type openRoomSessionPresence struct {
	presence interface {
		studentpresence.OpenRoomSessionQuery
		QueryLiveGroups(context.Context, studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error)
		QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
	}
	activities openRoomActivityQuery
}

func (s openRoomSessionPresence) GetStaffActiveGroupIDs(ctx context.Context, staffID int64) ([]int64, error) {
	return activeStaffGroupIDs(ctx, s.presence, staffID)
}

func (s openRoomSessionPresence) ListRunningSessionIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.presence.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{})
	if err != nil {
		return nil, &studentpresence.OperationError{Op: "ListActiveGroups", Err: fmt.Errorf("list failed: %w", err)}
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.IsOpen() {
			ids = append(ids, row.ID)
		}
	}
	return ids, nil
}

func (s openRoomSessionPresence) FindOpenSessionsInRooms(ctx context.Context, ids []int64) ([]supervisiondashboard.RunningSession, error) {
	rows, err := s.presence.ListOpenRoomSessions(ctx, ids)
	if err != nil {
		return nil, err
	}
	activityIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.ActivityGroupID != nil {
			activityIDs = append(activityIDs, *row.ActivityGroupID)
		}
	}
	activities := make(map[int64]timetable.Group, len(activityIDs))
	if len(activityIDs) > 0 {
		groups, err := s.activities.ListGroups(ctx, timetable.GroupFilter{IDs: activityIDs})
		if err != nil {
			return nil, err
		}
		for _, group := range groups {
			activities[group.ID] = group
		}
	}
	result := make([]supervisiondashboard.RunningSession, 0, len(rows))
	for _, row := range rows {
		session := supervisiondashboard.RunningSession{ActiveGroupID: row.ActiveGroupID, RoomID: row.RoomID, StartTime: row.StartTime, SupervisorStaffIDs: row.SupervisorStaffIDs}
		if row.ActivityGroupID != nil {
			activity := activities[*row.ActivityGroupID]
			session.ActivityName = activity.Name
			// Independent stays are device-less system sessions (#3066). A
			// kiosk-owned Schulhof Freispiel is a real supervision, not one.
			session.IndependentStays = (&activeModels.Group{
				GroupID:  row.ActivityGroupID,
				DeviceID: row.DeviceID,
			}).IsIndependentRoomSession(activity.IsSystem)
		}
		result = append(result, session)
	}
	return result, nil
}
