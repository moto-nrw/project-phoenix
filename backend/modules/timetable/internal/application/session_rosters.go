package application

import (
	"context"
	"maps"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// SessionsRosteringStudent maps the running sessions to their blocks through
// Student Presence and keeps those whose day roster lists the child. A
// Student Presence error passes through unchanged; the roster read maps its
// errors like every instance student read.
func (s *Service) SessionsRosteringStudent(ctx context.Context, studentID int64, sessionIDs []int64) ([]int64, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}
	sessionByInstance, err := s.sessions.RunningSessionsByInstance(ctx, sessionIDs)
	if err != nil {
		return nil, err
	}
	if len(sessionByInstance) == 0 {
		return nil, nil
	}
	rows, err := s.listInstanceStudents(ctx, timetable.InstanceStudentFilter{
		InstanceIDs: slices.Sorted(maps.Keys(sessionByInstance)), StudentIDs: []int64{studentID},
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
