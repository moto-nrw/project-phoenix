package services

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
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
		return nil, &activeService.ActiveError{Op: "ListActiveGroups", Err: fmt.Errorf("list failed: %w", err)}
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
	names := make(map[int64]string, len(activityIDs))
	if len(activityIDs) > 0 {
		groups, err := s.activities.ListGroups(ctx, timetable.GroupFilter{IDs: activityIDs})
		if err != nil {
			return nil, err
		}
		for _, group := range groups {
			names[group.ID] = group.Name
		}
	}
	result := make([]supervisiondashboard.RunningSession, 0, len(rows))
	for _, row := range rows {
		session := supervisiondashboard.RunningSession{ActiveGroupID: row.ActiveGroupID, RoomID: row.RoomID, StartTime: row.StartTime, SupervisorStaffIDs: row.SupervisorStaffIDs}
		if row.ActivityGroupID != nil {
			session.ActivityName = names[*row.ActivityGroupID]
		}
		result = append(result, session)
	}
	return result, nil
}
