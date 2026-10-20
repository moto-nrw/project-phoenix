package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// EndedGroupSession is what one group session end closed.
type EndedGroupSession struct {
	GroupID            int64
	EndedAt            time.Time
	ClosedVisits       []*studentpresence.Visit
	EndedSupervisorIDs []int64
}

// GroupSessionStore closes live groups inside the caller's transaction.
type GroupSessionStore interface {
	LockSessionStart(context.Context, int64) (Stats, error)
	LockGroup(context.Context, int64) (studentpresence.LiveGroup, Stats, error)
	EndGroupSession(context.Context, int64, time.Time, Date) (EndedGroupSession, Stats, error)
	EndGroupSessions(context.Context, []int64, time.Time, Date) (studentpresence.EndedGroupSessions, Stats, error)
	EndGroup(context.Context, int64, time.Time) (Stats, error)
}
