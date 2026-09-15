package workforce

import (
	"context"
	"errors"
	"time"
)

// Time-tracking failures. The wording of a wrapped cause is the message the
// retained service produced and the HTTP resources render; the kind is what
// they classify on.
var (
	// ErrTimeTrackingConflict reports a stamp that does not fit the current
	// state: already checked in, already checked out today, break already
	// active.
	ErrTimeTrackingConflict = errors.New("time tracking state conflict")
	// ErrTimeTrackingNotFound reports a missing block or break: no active
	// session found, no session found for today, session not found, no
	// active break found.
	ErrTimeTrackingNotFound = errors.New("time tracking record not found")
	// ErrTimeTrackingForbidden reports a session that belongs to somebody
	// else.
	ErrTimeTrackingForbidden = errors.New("time tracking record belongs to another staff member")
	// ErrTimeTrackingInvalid reports input the retained service rejected
	// before any write.
	ErrTimeTrackingInvalid = errors.New("invalid time tracking input")
	// ErrWorkSessionOverlap reports a stamp that falls inside a closed block.
	// The message carries the conflicting interval, so it is unusable as a
	// mapping key; consumers use this kind instead.
	ErrWorkSessionOverlap = errors.New("work session overlaps an existing block")
	// ErrCheckInRaced reports that a concurrent stamp of the same staff member
	// already opened today's block; the request transaction has been marked
	// for rollback because the duplicate insert aborted it.
	ErrCheckInRaced = errors.New("check-in for today was already recorded")
)

// PlannedStartNotReachedError reports a check-in before the planned start of
// the day while the tenant enforces the planned start. Times are Berlin wall
// clocks in "15:04".
type PlannedStartNotReachedError struct {
	PlannedStartTime string
	CurrentTime      string
}

func (e *PlannedStartNotReachedError) Error() string { return "planned start not reached" }

// DeviationReasonRequiredError reports a stamp outside the tolerance window
// around the planned shift window without the reason the tenant requires
// (F9). Action is check_in or check_out; the times are Berlin wall clocks.
type DeviationReasonRequiredError struct {
	Action           string
	PlannedTime      string
	ActualTime       string
	DeviationMinutes int
}

func (e *DeviationReasonRequiredError) Error() string { return "deviation reason required" }

// TimeTrackingError carries a classified failure of the retained time-tracking
// services. Error returns the original wording, Is reports the kind, and
// Unwrap keeps the cause chain reachable.
type TimeTrackingError struct {
	Kind  error
	Cause error
}

func (e *TimeTrackingError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Kind.Error()
}

func (e *TimeTrackingError) Is(target error) bool { return target == e.Kind }
func (e *TimeTrackingError) Unwrap() error        { return e.Cause }

// LaborTimeEvaluation is the as-of-now view of a workday under the labor-law
// break rules (§4 ArbZG) every reader shares, so the kiosk never reimplements
// the thresholds.
type LaborTimeEvaluation struct {
	NetMinutes           int
	BreakMinutes         int
	RequiredBreakMinutes int
	IsBreakCompliant     bool
}

// Workday is one calendar day of a staff member as the kiosk sees it: every
// block intersecting the day in check-in order, the breaks of each block
// keyed by session, and the labor-time figures as of the requested instant.
// An open block that passed its live limit is cut at that instant and shows
// up as the closed block it effectively is; one whose limit lies before the
// day begins is left out.
type Workday struct {
	Sessions  []WorkSession
	Breaks    map[int64][]WorkSessionBreak
	LaborTime LaborTimeEvaluation
}

// CheckInStamp is one check-in. Day pins the calendar day a kiosk stamp acts
// on; empty means the current day. Reason is the optional F9 deviation
// reason.
type CheckInStamp struct {
	StaffID int64
	Day     string
	Status  string
	Source  string
	Reason  string
}

// TimeClock is the stamp contract of the web app and the kiosk. Every write
// runs on the caller's ambient tenant transaction. The *On variants act on the
// open block of an explicit calendar day so a request that straddles Berlin
// midnight cannot write on one day and read on the next.
type TimeClock interface {
	CheckIn(context.Context, CheckInStamp) (WorkSession, error)
	CheckOut(ctx context.Context, staffID int64, reason string) (WorkSession, error)
	StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (WorkSessionBreak, error)
	EndBreak(ctx context.Context, staffID int64) (WorkSession, error)
	CheckOutOn(ctx context.Context, staffID int64, day, reason string) (WorkSession, error)
	StartBreakOn(ctx context.Context, staffID int64, day string, plannedDurationMinutes *int) (WorkSessionBreak, error)
	EndBreakOn(ctx context.Context, staffID int64, day string) (WorkSession, error)
	// LatestOpenSession returns the block still running inside the live
	// window, whatever day it was opened on; found is false when there is
	// none.
	LatestOpenSession(ctx context.Context, staffID int64) (WorkSession, bool, error)
	// Workday returns the day's blocks, breaks and labor-time figures as of
	// now.
	Workday(ctx context.Context, staffID int64, day string, now time.Time) (Workday, error)
}
