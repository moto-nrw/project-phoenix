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

// GroupSessionCommand closes one live group and everything still open in
// it. Both operations require the caller's tenant transaction: the session
// end workflow holds the group lock from the first read to the commit.
type GroupSessionCommand interface {
	// LockGroup reads the group FOR UPDATE and returns ErrGroupNotFound when
	// the tenant has no such row.
	LockGroup(context.Context, int64) (LiveGroup, error)
	// EndGroupSession closes every open visit and supervision of the group at
	// the given instant and stamps the group's end time. An already ended
	// group fails with ErrGroupEnded; a missing one with ErrGroupNotFound.
	EndGroupSession(context.Context, int64, time.Time) (EndedGroupSession, error)
}

func (m *Module) LockGroup(ctx context.Context, activeGroupID int64) (LiveGroup, error) {
	return m.engine.LockGroup(ctx, activeGroupID)
}

func (m *Module) EndGroupSession(ctx context.Context, activeGroupID int64, at time.Time) (EndedGroupSession, error) {
	return m.engine.EndGroupSession(ctx, activeGroupID, at)
}
