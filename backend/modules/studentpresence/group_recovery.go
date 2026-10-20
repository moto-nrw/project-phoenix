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
