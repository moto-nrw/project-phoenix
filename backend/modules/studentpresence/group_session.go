package studentpresence

import (
	"context"
	"time"
)

// LiveGroup is the owner's view of one active.groups row: the session a
// device or a planner opened in a room. Room and activity display data remain
// with their owners.
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

// IsOpen reports whether the session has not ended yet.
func (g LiveGroup) IsOpen() bool { return g.EndTime == nil }

// EndedGroupSession records what one session end closed.
type EndedGroupSession struct {
	GroupID            int64
	EndedAt            time.Time
	ClosedVisits       []Visit
	EndedSupervisorIDs []int64
}

// EndedGroupSessions counts what a bulk session end closed.
type EndedGroupSessions struct {
	VisitsClosed        int64
	SessionsEnded       int64
	SupervisorsEnded    int64
	EndedActiveGroupIDs []int64
}

// GroupSessionCommand closes live groups and everything still open in them.
// Every operation requires the caller's tenant transaction: the session end
// workflow holds the group lock from the first read to the commit.
type SessionStartCommand interface {
	// LockSessionStart serializes starts for one activity until the caller's tenant transaction ends.
	LockSessionStart(context.Context, int64) error
}

type GroupSessionCommand interface {
	SessionStartCommand
	// LockGroup reads the group FOR UPDATE and returns ErrGroupNotFound when
	// the tenant has no such row.
	LockGroup(context.Context, int64) (LiveGroup, error)
	// EndGroupSession closes every open visit and supervision of the group at
	// the given instant and stamps the group's end time. An already ended
	// group fails with ErrGroupEnded; a missing one with ErrGroupNotFound.
	EndGroupSession(context.Context, int64, time.Time) (EndedGroupSession, error)
	// EndGroupSessions is the nightly bulk close: it ends the still-open
	// groups among the given IDs together with their open visits and
	// supervisions in three statements. Visit exits are clamped to the entry
	// time, as the nightly job always did. Groups already ended are skipped,
	// not rejected. A fourth statement compacts the visit revision events of
	// ended sessions into one row each, keeping every revision (#5128).
	// This tenant-wide maintenance also runs for an empty list or when all
	// supplied groups have already ended; deleted sessions' events are removed.
	EndGroupSessions(context.Context, []int64, time.Time) (EndedGroupSessions, error)
	// EndGroup releases a live group for a takeover: it stamps the group's
	// end time and nothing else, so the caller can move the still-open
	// visits and supervisions to the group that replaces it (force start,
	// absorption into a started instance). It joins the caller's transaction
	// or runs in its own, like the visit transfer beside it. An already ended
	// group fails with ErrGroupEnded; a missing one with ErrGroupNotFound.
	EndGroup(context.Context, int64, time.Time) error
}
