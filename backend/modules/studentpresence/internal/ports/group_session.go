package ports

import (
	"context"
	"time"
)

// LiveGroup mirrors one active.groups row for the session end command.
type LiveGroup struct {
	ID, TenantID            int64
	CreatedAt, UpdatedAt    time.Time
	StartTime, LastActivity time.Time
	EndTime                 *time.Time
	TimeoutMinutes          int
	ActivityGroupID         *int64
	DeviceID                *int64
	RoomID                  int64
}

// EndedGroupSession is what one group session end closed.
type EndedGroupSession struct {
	GroupID            int64
	EndedAt            time.Time
	ClosedVisits       []*Visit
	EndedSupervisorIDs []int64
}

// EndedGroupSessions counts what a bulk close changed.
type EndedGroupSessions struct {
	VisitsClosed        int64
	SessionsEnded       int64
	SupervisorsEnded    int64
	EndedActiveGroupIDs []int64
}

// GroupSessionStore closes live groups inside the caller's transaction.
type GroupSessionStore interface {
	LockGroup(context.Context, int64) (LiveGroup, Stats, error)
	EndGroupSession(context.Context, int64, time.Time, Date) (EndedGroupSession, Stats, error)
	EndGroupSessions(context.Context, []int64, time.Time, Date) (EndedGroupSessions, Stats, error)
	EndGroup(context.Context, int64, time.Time) (Stats, error)
}
