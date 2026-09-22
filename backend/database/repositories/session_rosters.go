package repositories

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// SessionRosters serves the device-scan roster port: which running sessions list a child in their day
// roster from the two owners of a block since #2762: Student Presence knows
// which block a session runs, Timetable knows who is planned on it.
type SessionRosters struct {
	Sessions interface {
		ListActivitySessions(context.Context, studentpresence.ActivitySessionFilter) ([]studentpresence.ActivitySession, error)
	}
	Roster interface {
		ListInstanceStudents(context.Context, timetable.InstanceStudentFilter) ([]timetable.InstanceStudent, error)
	}
}

func (r SessionRosters) SessionsRosteringStudent(ctx context.Context, studentID int64, sessionIDs []int64) ([]int64, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}
	sessions, err := r.Sessions.ListActivitySessions(ctx, studentpresence.ActivitySessionFilter{
		ActiveGroupIDs: sessionIDs, Status: studentpresence.ActivitySessionActive,
	})
	if err != nil {
		return nil, err
	}
	sessionByInstance := make(map[int64]int64, len(sessions))
	instanceIDs := make([]int64, 0, len(sessions))
	for _, session := range sessions {
		if session.ActiveGroupID == nil {
			continue
		}
		sessionByInstance[session.InstanceID] = *session.ActiveGroupID
		instanceIDs = append(instanceIDs, session.InstanceID)
	}
	if len(instanceIDs) == 0 {
		return nil, nil
	}
	rows, err := r.Roster.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{
		InstanceIDs: instanceIDs, StudentIDs: []int64{studentID},
	})
	if err != nil {
		return nil, err
	}
	result := make([]int64, 0, len(rows))
	for _, row := range rows {
		if sessionID, ok := sessionByInstance[row.InstanceID]; ok {
			result = append(result, sessionID)
		}
	}
	return result, nil
}
