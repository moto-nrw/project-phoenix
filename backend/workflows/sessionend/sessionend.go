// Package sessionend is the application workflow that closes one live
// activity session ("Sitzung beenden"). It coordinates the Student Presence
// owner (group, visits, supervisions) and the Timetable & Activities owner
// (mirrored instance, slot check-outs, attendance finalization) in one
// UnitOfWork and announces the close only after that unit committed.
package sessionend

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrSessionNotFound reports that the tenant has no such live group. The
	// texts keep the strings the kiosk endpoint rendered before the cutover.
	ErrSessionNotFound = errors.New("active group not found")
	// ErrSessionAlreadyEnded reports that the group was closed before.
	ErrSessionAlreadyEnded = errors.New("active group session already ended")
)

// Result describes what one session end changed.
type Result struct {
	ActiveGroupID      int64
	EndedAt            time.Time
	StudentsCheckedOut int
	SupervisorsEnded   int
	// MirroredInstanceID names the timetable instance this close completed.
	// It is nil when the session was never mirrored into the timetable or
	// when another path had already completed the instance.
	MirroredInstanceID *int64
}

// Command ends a live session. The command joins the caller's tenant
// transaction when one is open and otherwise runs its own; every write of the
// close belongs to that single unit. When the caller owns the transaction, a
// returned error means the caller must roll back: nothing of the close may
// survive on its own.
type Command interface {
	EndSession(ctx context.Context, activeGroupID int64) (Result, error)
}
