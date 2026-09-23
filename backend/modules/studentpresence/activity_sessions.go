package studentpresence

import (
	"context"
	"errors"
	"time"
)

// Activity session status values. A planned or cancelled occurrence has no
// session; the session exists from the start of the block to its completion
// and stays as the completed record.
const (
	ActivitySessionActive    = "active"
	ActivitySessionCompleted = "completed"
)

var (
	ErrActivitySessionNotFound = errors.New("activity session not found")
	ErrActivitySessionExists   = errors.New("activity session already exists")
)

// ActivitySession is the owner's view of one active.activity_sessions row:
// the execution of a planned activity instance (#2762). The instance itself,
// its date, times, room and roster stay with Timetable & Activities.
type ActivitySession struct {
	ID, TenantID         int64
	CreatedAt, UpdatedAt time.Time
	InstanceID           int64
	Status               string
	ActiveGroupID        *int64
	StartedBy            *int64
	StartedAt            *time.Time
	CompletedAt          *time.Time
	// CompletedBy is the global auth account that completed the block, not a
	// staff id; the reopen window is granted to that account.
	CompletedBy        *int64
	ReopenUntil        *time.Time
	CompletionSnapshot []byte
}

// IsActive reports whether the block is running.
func (s ActivitySession) IsActive() bool { return s.Status == ActivitySessionActive }

type ActivitySessionFilter struct {
	InstanceIDs    []int64
	ActiveGroupIDs []int64
	Status         string
}

// SessionExecutionFilter names the instances and the participants one
// execution read answers for.
type SessionExecutionFilter struct {
	InstanceIDs    []int64
	ParticipantIDs []int64
}

// SessionExecution is that answer: the instances that ended and the
// participants that carry the non-booking marker.
type SessionExecution struct {
	CompletedInstanceIDs       []int64
	NotScheduledParticipantIDs []int64
}

// ActivitySessionStart opens the session of a planned instance.
type ActivitySessionStart struct {
	InstanceID    int64
	ActiveGroupID int64
	StartedBy     *int64
	StartedAt     time.Time
}

// ActivitySessionCompletion closes a running session. CompletionSnapshot is
// the reopen evidence the completing workflow captured.
type ActivitySessionCompletion struct {
	InstanceID         int64
	CompletedAt        time.Time
	CompletedBy        *int64
	ReopenUntil        *time.Time
	CompletionSnapshot []byte
}

type ActivitySessionQuery interface {
	// FindActivitySession returns the session of the instance or
	// ErrActivitySessionNotFound when the instance has not been started.
	FindActivitySession(context.Context, int64) (ActivitySession, error)
	ListActivitySessions(context.Context, ActivitySessionFilter) ([]ActivitySession, error)
	// SessionExecution answers in one read which of the instances ended and
	// which of the participants are marked as not booked.
	SessionExecution(context.Context, SessionExecutionFilter) (SessionExecution, error)
}

type ActivitySessionCommand interface {
	// StartActivitySession records the start of a planned instance. A second
	// start of the same instance fails with ErrActivitySessionExists.
	StartActivitySession(context.Context, ActivitySessionStart) (ActivitySession, error)
	// CompleteActivitySession closes the running session of the instance and
	// stores the reopen evidence. A missing or already completed session
	// fails with ErrActivitySessionNotFound.
	CompleteActivitySession(context.Context, ActivitySessionCompletion) (ActivitySession, error)
	// RecordActivitySessionCompleted stamps an instance completed at the given
	// instant regardless of whether it was started, as the nightly close
	// always did; a session without a start is created completed.
	RecordActivitySessionCompleted(context.Context, int64, time.Time) error
	// ReopenActivitySession returns exactly the completed session of the
	// instance to active on the given live group and clears the completion.
	// A mismatch fails with ErrActivitySessionNotFound so the surrounding
	// recovery transaction aborts.
	ReopenActivitySession(context.Context, int64, int64) (ActivitySession, error)
	// CompleteActivitySessionsByGroups is the nightly bulk close: every
	// running session on one of the given live groups is completed at the
	// instant. It returns how many it closed.
	CompleteActivitySessionsByGroups(context.Context, []int64, time.Time) (int64, error)
	// DiscardActivitySession removes the session of an instance whose execution
	// is being undone, such as a cancelled running block. Callers end the
	// session before they change the planning status.
	DiscardActivitySession(context.Context, int64) error
}

func (m *Module) FindActivitySession(ctx context.Context, instanceID int64) (ActivitySession, error) {
	return m.engine.FindActivitySession(ctx, instanceID)
}

func (m *Module) ListActivitySessions(ctx context.Context, filter ActivitySessionFilter) ([]ActivitySession, error) {
	return m.engine.ListActivitySessions(ctx, filter)
}

func (m *Module) SessionExecution(ctx context.Context, filter SessionExecutionFilter) (SessionExecution, error) {
	return m.engine.SessionExecution(ctx, filter)
}

func (m *Module) StartActivitySession(ctx context.Context, start ActivitySessionStart) (ActivitySession, error) {
	return m.engine.StartActivitySession(ctx, start)
}

func (m *Module) CompleteActivitySession(ctx context.Context, completion ActivitySessionCompletion) (ActivitySession, error) {
	return m.engine.CompleteActivitySession(ctx, completion)
}

func (m *Module) RecordActivitySessionCompleted(ctx context.Context, instanceID int64, at time.Time) error {
	return m.engine.RecordActivitySessionCompleted(ctx, instanceID, at)
}

func (m *Module) ReopenActivitySession(ctx context.Context, instanceID, activeGroupID int64) (ActivitySession, error) {
	return m.engine.ReopenActivitySession(ctx, instanceID, activeGroupID)
}

func (m *Module) CompleteActivitySessionsByGroups(ctx context.Context, activeGroupIDs []int64, at time.Time) (int64, error) {
	return m.engine.CompleteActivitySessionsByGroups(ctx, activeGroupIDs, at)
}

func (m *Module) DiscardActivitySession(ctx context.Context, instanceID int64) error {
	return m.engine.DiscardActivitySession(ctx, instanceID)
}
