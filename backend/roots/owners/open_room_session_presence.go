package owners

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type openRoomActivityQuery interface {
	ListGroups(context.Context, timetable.GroupFilter) ([]timetable.Group, error)
}

type OpenRoomSessionPresence struct {
	Presence interface {
		studentpresence.OpenRoomSessionQuery
		QueryLiveGroups(context.Context, studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error)
		QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
	}
	Activities openRoomActivityQuery
}

func (s OpenRoomSessionPresence) GetStaffActiveGroupIDs(ctx context.Context, staffID int64) ([]int64, error) {
	return activeStaffGroupIDs(ctx, s.Presence, staffID)
}

func (s OpenRoomSessionPresence) ListRunningSessionIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.Presence.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{})
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

func (s OpenRoomSessionPresence) FindOpenSessionsInRooms(ctx context.Context, ids []int64) ([]supervisiondashboard.RunningSession, error) {
	rows, err := s.Presence.ListOpenRoomSessions(ctx, ids)
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
		groups, err := s.Activities.ListGroups(ctx, timetable.GroupFilter{IDs: activityIDs})
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
			session.ParticipantLimit = timetable.ParticipantLimitPtr(activity.MaxParticipants)
			// Independent stays are device-less system sessions (#3066). A
			// kiosk-owned Schulhof Freispiel is a real supervision, not one.
			session.IndependentStays = studentpresence.LiveGroup{
				ActivityGroupID: row.ActivityGroupID,
				DeviceID:        row.DeviceID,
			}.IsIndependentRoomSession(activity.IsSystem)
		}
		result = append(result, session)
	}
	return result, nil
}

// SessionSupervisorStaffIDs maps the sessions to the staff supervising them
// today in one read (#4354).
func (s OpenRoomSessionPresence) SessionSupervisorStaffIDs(ctx context.Context, sessionIDs []int64) (map[int64][]int64, error) {
	result := make(map[int64][]int64, len(sessionIDs))
	if len(sessionIDs) == 0 {
		return result, nil
	}
	rows, err := supervisionsToday(ctx, s.Presence, studentpresence.GroupSupervisionFilter{GroupIDs: sessionIDs}, "SessionSupervisorStaffIDs")
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.GroupID] = append(result[row.GroupID], row.StaffID)
	}
	return result, nil
}

type groupSupervisionQuery interface {
	QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
}

func activeStaffGroupIDs(ctx context.Context, presence groupSupervisionQuery, staffID int64) ([]int64, error) {
	rows, err := supervisionsToday(ctx, presence, studentpresence.GroupSupervisionFilter{StaffID: &staffID}, "GetStaffActiveSupervisions")
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.GroupID)
	}
	return ids, nil
}

// supervisionsToday reads the supervisions running today under the filter.
// ActiveOn is the owner's rule: started on or before the day, not ended by it.
func supervisionsToday(ctx context.Context, presence groupSupervisionQuery, filter studentpresence.GroupSupervisionFilter, op string) ([]studentpresence.GroupSupervision, error) {
	day := timezone.TodayDate().String()
	filter.ActiveOn = &day
	rows, err := presence.QueryGroupSupervisions(ctx, filter)
	if err != nil {
		return nil, &studentpresence.OperationError{Op: op, Err: studentpresence.ErrDatabaseOperation}
	}
	return rows, nil
}
