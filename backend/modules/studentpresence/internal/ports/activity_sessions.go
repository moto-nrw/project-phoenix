package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// Session statuses as stored in active.activity_sessions.status.
const (
	ActivitySessionActive    = "active"
	ActivitySessionCompleted = "completed"
)

// ActivitySessionStore persists active.activity_sessions.
type ActivitySessionStore interface {
	FindActivitySession(context.Context, int64) (studentpresence.ActivitySession, bool, Stats, error)
	ListActivitySessions(context.Context, studentpresence.ActivitySessionFilter) ([]studentpresence.ActivitySession, Stats, error)
	SessionExecution(context.Context, studentpresence.SessionExecutionFilter) (studentpresence.SessionExecution, Stats, error)
	StartActivitySession(context.Context, studentpresence.ActivitySessionStart) (studentpresence.ActivitySession, Stats, error)
	CompleteActivitySession(context.Context, studentpresence.ActivitySessionCompletion) (studentpresence.ActivitySession, bool, Stats, error)
	RecordActivitySessionCompleted(context.Context, int64, time.Time) (Stats, error)
	ReopenActivitySession(context.Context, int64, int64) (studentpresence.ActivitySession, bool, Stats, error)
	CompleteActivitySessionsByGroups(context.Context, []int64, time.Time) (Stats, error)
	DiscardActivitySession(context.Context, int64) (Stats, error)
}
