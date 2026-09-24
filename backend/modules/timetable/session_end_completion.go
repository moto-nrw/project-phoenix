package timetable

import (
	"context"
	"time"
)

// EndedSessionCompletion completes the blocks of live sessions somebody else
// already ended (the nightly session end, a force start, the kiosk's
// "Sitzung beenden"), and finalizes their attendance first: children the
// care plan expects flip expected → absent, children it does not book that
// day keep their row and carry the frozen not-scheduled marker (#1747). No
// caller can stamp such a block completed without that finalization.
type EndedSessionCompletion interface {
	// CompleteActiveByActiveGroupIDs completes every still-running block
	// bridged to one of the sessions and returns how many it completed.
	CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error)
}
