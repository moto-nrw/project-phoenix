package ports

import (
	"context"
	"errors"
	"time"
)

var (
	ErrActivitySessionNotFound = errors.New("activity session not found")
	ErrActivitySessionExists   = errors.New("activity session already exists")
)

// ActivitySession is the persistence-independent execution record of one
// planned activity instance.
type ActivitySession struct {
	ID, TenantID         int64
	CreatedAt, UpdatedAt time.Time
	InstanceID           int64
	Status               string
	ActiveGroupID        *int64
	StartedBy            *int64
	StartedAt            *time.Time
	CompletedAt          *time.Time
	CompletedBy          *int64
	ReopenUntil          *time.Time
	CompletionSnapshot   []byte
}

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

type ActivitySessionStart struct {
	InstanceID    int64
	ActiveGroupID int64
	StartedBy     *int64
	StartedAt     time.Time
}

type ActivitySessionCompletion struct {
	InstanceID         int64
	CompletedAt        time.Time
	CompletedBy        *int64
	ReopenUntil        *time.Time
	CompletionSnapshot []byte
}

// ActivitySessionStore persists active.activity_sessions.
type ActivitySessionStore interface {
	FindActivitySession(context.Context, int64) (ActivitySession, bool, Stats, error)
	ListActivitySessions(context.Context, ActivitySessionFilter) ([]ActivitySession, Stats, error)
	SessionExecution(context.Context, SessionExecutionFilter) (SessionExecution, Stats, error)
	StartActivitySession(context.Context, ActivitySessionStart) (ActivitySession, Stats, error)
	CompleteActivitySession(context.Context, ActivitySessionCompletion) (ActivitySession, bool, Stats, error)
	RecordActivitySessionCompleted(context.Context, int64, time.Time) (Stats, error)
	ReopenActivitySession(context.Context, int64, int64) (ActivitySession, bool, Stats, error)
	CompleteActivitySessionsByGroups(context.Context, []int64, time.Time) (Stats, error)
	DiscardActivitySession(context.Context, int64) (Stats, error)
}
