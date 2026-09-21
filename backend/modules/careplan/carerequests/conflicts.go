package carerequests

import (
	"context"
	"errors"
	"time"
)

var ErrStaffValueUnsupported = errors.New("parent requests: this request kind accepts no staff-entered value")

type ConflictCandidate struct {
	StudentID int64
	UpdatedAt time.Time
}

type StaffValueWrite struct {
	StudentID   int64
	RequestIDs  []int64
	ConflictKey string
	Reason      string
	PickupTime  string
}

// Conflicts contributes owner decisions to an already-authorized, locked
// conflict group. The coordinator supplies one child and one owner-derived key.
// All writes run in the coordinator's ambient tenant transaction.
type Conflicts interface {
	ConflictCandidate(context.Context, int64) (*ConflictCandidate, error)
	LockConflictRequest(context.Context, int64) error
	DecideConflictRequest(context.Context, DecideInput) error
	WriteStaffValue(context.Context, StaffValueWrite) error
}
