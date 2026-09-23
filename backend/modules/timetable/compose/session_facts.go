package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// PresenceSessions is the slice of the Student Presence query surface the
// session facts read.
type PresenceSessions interface {
	ListActivitySessions(context.Context, studentpresence.ActivitySessionFilter) ([]studentpresence.ActivitySession, error)
	ListSessionAttendance(context.Context, []int64) ([]studentpresence.SessionAttendance, error)
	SessionExecution(context.Context, studentpresence.SessionExecutionFilter) (studentpresence.SessionExecution, error)
}

// NewPresenceSessionFacts serves Timetable's SessionFacts port from the
// Student Presence owner: which planned blocks already run or ended, which
// participants were observed or marked as not booked (#2762).
func NewPresenceSessionFacts(presence PresenceSessions) timetable.SessionFacts {
	if presence == nil {
		panic("presence session facts: student presence is required")
	}
	return presenceSessionFacts{presence: presence}
}

type presenceSessionFacts struct{ presence PresenceSessions }

func (f presenceSessionFacts) sessionInstanceIDs(ctx context.Context, instanceIDs []int64, status string) ([]int64, error) {
	if len(instanceIDs) == 0 {
		return nil, nil
	}
	sessions, err := f.presence.ListActivitySessions(ctx, studentpresence.ActivitySessionFilter{InstanceIDs: instanceIDs, Status: status})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.InstanceID)
	}
	return ids, nil
}

func (f presenceSessionFacts) StartedInstanceIDs(ctx context.Context, instanceIDs []int64) ([]int64, error) {
	return f.sessionInstanceIDs(ctx, instanceIDs, "")
}

func (f presenceSessionFacts) CompletedInstanceIDs(ctx context.Context, instanceIDs []int64) ([]int64, error) {
	return f.sessionInstanceIDs(ctx, instanceIDs, studentpresence.ActivitySessionCompleted)
}

func (f presenceSessionFacts) attendanceParticipantIDs(ctx context.Context, participantIDs []int64, matches func(studentpresence.SessionAttendance) bool) ([]int64, error) {
	if len(participantIDs) == 0 {
		return nil, nil
	}
	rows, err := f.presence.ListSessionAttendance(ctx, participantIDs)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if matches(row) {
			ids = append(ids, row.ParticipantID)
		}
	}
	return ids, nil
}

func (f presenceSessionFacts) ObservedParticipantIDs(ctx context.Context, participantIDs []int64) ([]int64, error) {
	return f.attendanceParticipantIDs(ctx, participantIDs, func(row studentpresence.SessionAttendance) bool {
		return row.CheckedInAt != nil || row.CheckedOutAt != nil
	})
}

func (f presenceSessionFacts) NotScheduledParticipantIDs(ctx context.Context, participantIDs []int64) ([]int64, error) {
	return f.attendanceParticipantIDs(ctx, participantIDs, func(row studentpresence.SessionAttendance) bool {
		return row.NotScheduled
	})
}

func (f presenceSessionFacts) ExecutionFacts(ctx context.Context, instanceIDs, participantIDs []int64) ([]int64, []int64, error) {
	if len(instanceIDs) == 0 && len(participantIDs) == 0 {
		return nil, nil, nil
	}
	result, err := f.presence.SessionExecution(ctx, studentpresence.SessionExecutionFilter{InstanceIDs: instanceIDs, ParticipantIDs: participantIDs})
	if err != nil {
		return nil, nil, err
	}
	return result.CompletedInstanceIDs, result.NotScheduledParticipantIDs, nil
}
