package timetable

import "context"

// SessionRosterQuery is the part of the capability SessionRosters reads.
type SessionRosterQuery interface {
	ListActivityInstances(context.Context, ActivityInstanceFilter) ([]ActivityInstance, error)
	ListInstanceStudents(context.Context, InstanceStudentFilter) ([]InstanceStudent, error)
}

// SessionRosters resolves the day rosters of running blocks for the room
// sessions that carry them. The kiosk uses it to book a child scanned into
// the Schulhof into its own block (#3282).
type SessionRosters struct{ Query SessionRosterQuery }

// SessionsRosteringStudent returns those of sessionIDs whose active
// instance lists the student in its roster. A session without an active
// instance never matches.
func (r SessionRosters) SessionsRosteringStudent(ctx context.Context, studentID int64, sessionIDs []int64) ([]int64, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}
	instances, err := r.Query.ListActivityInstances(ctx, ActivityInstanceFilter{
		ActiveGroupIDs: sessionIDs, Status: InstanceStatusActive,
	})
	if err != nil {
		return nil, err
	}
	sessionByInstance := make(map[int64]int64, len(instances))
	instanceIDs := make([]int64, 0, len(instances))
	for _, instance := range instances {
		if instance.ActiveGroupID == nil {
			continue
		}
		sessionByInstance[instance.ID] = *instance.ActiveGroupID
		instanceIDs = append(instanceIDs, instance.ID)
	}
	if len(instanceIDs) == 0 {
		return nil, nil
	}
	rows, err := r.Query.ListInstanceStudents(ctx, InstanceStudentFilter{
		InstanceIDs: instanceIDs, StudentIDs: []int64{studentID},
	})
	if err != nil {
		return nil, err
	}
	sessions := make([]int64, 0, len(rows))
	for _, row := range rows {
		if sessionID, ok := sessionByInstance[row.InstanceID]; ok {
			sessions = append(sessions, sessionID)
		}
	}
	return sessions, nil
}
