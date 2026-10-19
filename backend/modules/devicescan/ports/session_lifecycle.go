package ports

import (
	"errors"
	"time"
)

// LifecycleSession is the session projection needed by the manual kiosk controls.
type LifecycleSession struct {
	ID, RoomID             int64
	ActivityID             *int64
	StartTime              time.Time
	ActivityName, RoomName *string
	Supervisors            []LifecycleSupervisor
}

type LifecycleSupervisor struct {
	StaffID int64
	Role    string
	Ended   bool
}

// SessionStartConflict preserves the start outcomes that require a second
// conflict lookup. Its cause retains the ordinary fallback classification.
type SessionStartConflict struct{ Cause error }

func (e *SessionStartConflict) Error() string { return e.Cause.Error() }

func (e *SessionStartConflict) Unwrap() error { return e.Cause }

// ErrLifecycleSessionAbsent distinguishes the normal no-session state.
var ErrLifecycleSessionAbsent = errors.New("kiosk session absent")
