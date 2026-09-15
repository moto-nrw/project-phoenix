package studentpresence

import (
	"context"
	"time"
)

// GroupRecovery covers the live-group and supervisor rows an activity
// completion snapshot closes and a reopen restores. Every operation requires
// the caller's tenant transaction so locks and restores commit together.
type GroupRecovery interface {
	LockOpenSupervisors(context.Context, int64) error
	LockSupervisors(context.Context, []int64) error
	RestoreGroup(context.Context, int64, time.Time) error
	RestoreSupervisors(context.Context, []int64) error
}

// LockOpenSupervisors holds every still-open supervisor of the group so
// completion snapshots the same rows the session end closes.
func (m *Module) LockOpenSupervisors(ctx context.Context, activeGroupID int64) error {
	return m.engine.LockOpenSupervisors(ctx, activeGroupID)
}

// LockSupervisors holds the snapshot supervisor rows during reopen so a
// concurrent staffing change cannot hide from the unchanged-row check.
func (m *Module) LockSupervisors(ctx context.Context, supervisorIDs []int64) error {
	return m.engine.LockSupervisors(ctx, supervisorIDs)
}

// RestoreGroup reopens exactly the ended active group in a completion
// snapshot. A mismatch fails the surrounding recovery transaction.
func (m *Module) RestoreGroup(ctx context.Context, activeGroupID int64, now time.Time) error {
	return m.engine.RestoreGroup(ctx, activeGroupID, now)
}

// RestoreSupervisors reopens exactly the ended supervisors in a completion
// snapshot. A mismatch fails the surrounding recovery transaction.
func (m *Module) RestoreSupervisors(ctx context.Context, supervisorIDs []int64) error {
	return m.engine.RestoreSupervisors(ctx, supervisorIDs)
}
