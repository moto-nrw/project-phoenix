package workforce

import (
	"context"
	"time"
)

const (
	SnapshotSourceAdmin     = "admin"
	SnapshotSourceScheduler = "scheduler"
	SnapshotSourceMigration = "migration"
)

// MonthSnapshots stores frozen carry balances and their reopening history.
// Reads exclude reopened snapshots. Writes join the caller's tenant transaction.
type MonthSnapshots interface {
	MonthSnapshotQuery
	MonthSnapshotCommand
	LockStaffBalanceWrites(context.Context, int64) error
}

type MonthSnapshotQuery interface {
	ClosedMonthSnapshotsForStaff(context.Context, []int64) ([]*StaffMonthBalanceSnapshot, error)
	LatestClosedMonth(context.Context, int64, int, int) (*StaffMonthBalanceSnapshot, error)
	ClosedMonthSnapshots(context.Context, int, int) ([]*StaffMonthBalanceSnapshot, error)
}

type MonthSnapshotCommand interface {
	RecordClosedMonth(context.Context, StaffMonthBalanceSnapshot) (StaffMonthBalanceSnapshot, error)
	ReopenMonthSnapshot(context.Context, int64, int64, time.Time, string) (int64, error)
}

type monthSnapshotEngine interface {
	MonthSnapshotQuery
	MonthSnapshotCommand
}

func (m *Module) LatestClosedMonth(ctx context.Context, staffID int64, year, month int) (*StaffMonthBalanceSnapshot, error) {
	return m.engine.LatestClosedMonth(ctx, staffID, year, month)
}

func (m *Module) ClosedMonthSnapshots(ctx context.Context, year, month int) ([]*StaffMonthBalanceSnapshot, error) {
	return m.engine.ClosedMonthSnapshots(ctx, year, month)
}

func (m *Module) RecordClosedMonth(ctx context.Context, value StaffMonthBalanceSnapshot) (StaffMonthBalanceSnapshot, error) {
	return m.engine.RecordClosedMonth(ctx, value)
}

func (m *Module) ReopenMonthSnapshot(ctx context.Context, id, actorID int64, at time.Time, reason string) (int64, error) {
	return m.engine.ReopenMonthSnapshot(ctx, id, actorID, at, reason)
}

func (s *StaffMonthBalanceSnapshot) IsActive() bool { return s.ReopenedAt == nil }

func (m *Module) ClosedMonthSnapshotsForStaff(ctx context.Context, staffIDs []int64) ([]*StaffMonthBalanceSnapshot, error) {
	return m.engine.ClosedMonthSnapshotsForStaff(ctx, staffIDs)
}
