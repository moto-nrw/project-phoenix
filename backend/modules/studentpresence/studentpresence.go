// Package studentpresence owns recorded school attendance and room visits.
package studentpresence

import (
	"context"
	"time"
)

type Query interface {
	UnclaimedGroups(context.Context, string) ([]UnclaimedGroup, error)
	AttendanceQuery
	VisitQuery
	ListOpenPresence(context.Context, []int64) ([]int64, error)
	LatestPresenceDate(context.Context, int64) (*string, error)
	CountAttendanceRecords(context.Context, int64) (int, error)
}

type Command interface {
	ClaimGroup(context.Context, GroupClaim) (ClaimedSupervision, error)
	AttendanceCommand
	VisitCommand
	GroupRecovery
	LockOpenPresence(context.Context, []int64) error
	CloseOpenPresence(context.Context, []int64, time.Time) (int64, error)
	LockOpenVisits(context.Context, int64) error
	RestoreVisits(context.Context, []int64) error
}

// LatestPresenceDate returns the last attendance or visit day as YYYY-MM-DD.
// Visit instants are interpreted in the school's Europe/Berlin calendar.
func (m *Module) LatestPresenceDate(ctx context.Context, studentID int64) (*string, error) {
	return m.engine.LatestPresenceDate(ctx, studentID)
}

// CountAttendanceRecords counts the student's attendance rows plus scheduled
// checkouts; the deletion preview reports both as attendance records.
func (m *Module) CountAttendanceRecords(ctx context.Context, studentID int64) (int, error) {
	return m.engine.CountAttendanceRecords(ctx, studentID)
}

func (m *Module) LockOpenPresence(ctx context.Context, studentIDs []int64) error {
	return m.engine.LockOpenPresence(ctx, studentIDs)
}

// CloseOpenPresence closes all recorded attendance and visits for a care exit.
// It joins the caller's transaction so roster and care-plan writes remain atomic.
func (m *Module) CloseOpenPresence(ctx context.Context, studentIDs []int64, at time.Time) (int64, error) {
	return m.engine.CloseOpenPresence(ctx, studentIDs, at)
}

type Capability interface {
	Query
	Command
}

// Module exposes presence operations without leaking persistence models.
type Module struct{ engine Capability }

func NewModule(engine Capability) *Module {
	if engine == nil {
		panic("student presence: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) ListOpenPresence(ctx context.Context, studentIDs []int64) ([]int64, error) {
	return m.engine.ListOpenPresence(ctx, studentIDs)
}

// LockOpenVisits holds the current group's visit locks until the surrounding
// workflow commits. It requires an existing tenant transaction.
func (m *Module) LockOpenVisits(ctx context.Context, activeGroupID int64) error {
	return m.engine.LockOpenVisits(ctx, activeGroupID)
}

// RestoreVisits reopens exactly the closed visits in a completion snapshot.
// A mismatch fails the surrounding recovery transaction.
func (m *Module) RestoreVisits(ctx context.Context, visitIDs []int64) error {
	return m.engine.RestoreVisits(ctx, visitIDs)
}
