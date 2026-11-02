package studentpresence

import (
	"context"
	"fmt"
)

// ErrVisitWriteConflict asks the caller to start a fresh command: the cohort
// changed while its visit locks were being acquired. No late visit is locked.
var ErrVisitWriteConflict = fmt.Errorf("%w: visit write cohort changed", ErrSessionConflict)

// VisitWriteScope identifies existing visits before any session or room row
// is locked. GroupIDs select open visits; SessionIDs lock only the sessions.
// DeviceID includes the device's running and recently ended sessions (one
// hour), and RoomID all running room sessions.
// RoomVisits also selects the room sessions' open visits.
// Explicit VisitIDs include closed intervals so reopening follows the order.
type VisitWriteScope struct {
	RoomVisits                                 bool
	VisitIDs, StudentIDs, GroupIDs, SessionIDs []int64
	DeviceID, RoomID                           int64
}

// VisitWriteLocks prepares a lifecycle/transfer command in its existing tenant
// transaction: visits by ID, room advisories by ID, then sessions by ID. It
// rechecks the cohort before returning. Call before any session INSERT too:
// its room foreign key already locks a room row. Scan does not need this lock.
type VisitWriteLocks interface {
	PrepareVisitWrites(context.Context, VisitWriteScope) error
}
