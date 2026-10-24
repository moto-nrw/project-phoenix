// Package staffclock defines the native NFC staff-clock workflow contract.
package staffclock

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The kiosk actions and states are the device-scan contract's; the aliases
// keep the workflow readable. The work session statuses and the stamp source
// are the Workforce capability's wire values.
const (
	ActionCheckIn    = devicescan.StaffClockActionCheckIn
	ActionCheckOut   = devicescan.StaffClockActionCheckOut
	ActionBreakStart = devicescan.StaffClockActionBreakStart
	ActionBreakEnd   = devicescan.StaffClockActionBreakEnd

	StateCheckedOut = devicescan.StaffClockStateCheckedOut
	StateCheckedIn  = devicescan.StaffClockStateCheckedIn
	StateOnBreak    = devicescan.StaffClockStateOnBreak

	StatusPresent    = workforce.WorkSessionStatusPresent
	StatusHomeOffice = workforce.WorkSessionStatusHomeOffice
	SourceNFC        = workforce.WorkSessionSourceNFC
)

var (
	ErrInvalidRFIDTag  = devicescan.ErrInvalidRFIDTag
	ErrRFIDTagNotFound = devicescan.ErrRFIDTagNotFound
	ErrRFIDTagInactive = devicescan.ErrRFIDTagInactive
	ErrRFIDTagNotStaff = devicescan.ErrRFIDTagNotStaff
	ErrInvalidAction   = devicescan.ErrInvalidAction
	ErrStatusRequired  = devicescan.ErrStatusRequired
	// ErrCheckInRaced reports that a concurrent scan of the same card already
	// created today's session. It is a state conflict, not a server fault.
	ErrCheckInRaced = devicescan.ErrStaffClockRaced
	// ErrStateConflict reports a stamp that does not fit the current block
	// state, including a missing block: rescan for authoritative state.
	ErrStateConflict = devicescan.ErrStaffClockState
	// ErrInvalidStamp reports stamp input the time clock refused.
	ErrInvalidStamp = devicescan.ErrStaffClockInvalid
)

// The command is the device-scan contract's; the typed stamp refusals carry
// their payload through to the kiosk unchanged.
type (
	StaffClockCommand            = devicescan.StaffClockCommand
	PlannedStartNotReachedError  = devicescan.PlannedStartNotReachedError
	DeviationReasonRequiredError = devicescan.DeviationReasonRequiredError
)

// StampError is a classified stamp refusal: Kind is one of the kiosk error
// kinds above and is what errors.Is reports, Cause keeps the wording of the
// time clock, which is what the kiosk shows.
type StampError struct {
	Kind  error
	Cause error
}

func (e *StampError) Error() string { return e.Cause.Error() }

func (e *StampError) Is(target error) bool { return target == e.Kind }

func (e *StampError) Unwrap() error { return e.Cause }

// Card is the RFID card projection the stamp workflow reads. The card table
// belongs to identity-access (#2662); the root adapts its repository to
// CardLookup so this package never imports that owner's models.
type Card struct {
	// ID is the normalized tag the staff lookup continues with.
	ID     string
	Active bool
}

// CardLookup resolves a scanned tag to a card of this school. The adapter
// normalizes the raw tag and reports a malformed one as ErrInvalidRFIDTag;
// an unknown tag is nil, nil.
type CardLookup interface {
	FindCard(ctx context.Context, rawTag string) (*Card, error)
}

// StaffIdentity is the staff member behind a card: the id every stamp is
// filed under and the name the kiosk greets with.
type StaffIdentity struct {
	StaffID  int64
	FullName string
}

// StaffLookup resolves a normalized tag to the staff member carrying it. A
// tag linked to nobody reports ErrRFIDTagNotFound, a person who is not staff
// ErrRFIDTagNotStaff; the adapter over People Directory decides both.
type StaffLookup interface {
	ResolveStaffByTag(ctx context.Context, tag string) (StaffIdentity, error)
}

// Session is the work block projection the kiosk renders. Day is the
// calendar day the block is filed on.
type Session struct {
	ID           int64
	StaffID      int64
	Day          string
	CheckInTime  time.Time
	CheckOutTime *time.Time
	Status       string
	Source       string
}

// IsOpen reports a block without a checkout.
func (s Session) IsOpen() bool { return s.CheckOutTime == nil }

// Break is one break of a block.
type Break struct {
	ID        int64
	StartedAt time.Time
	EndedAt   *time.Time
}

// IsActive reports a running break.
func (b Break) IsActive() bool { return b.EndedAt == nil }

// Workday is one calendar day of a staff member as the time clock sees it:
// the blocks, their breaks, and the labor-time figures of the whole day.
type Workday struct {
	Sessions []Session
	// Breaks are keyed by session id.
	Breaks               map[int64][]Break
	NetMinutes           int
	BreakMinutes         int
	RequiredBreakMinutes int
	IsBreakCompliant     bool
}

// Stamp is a check-in the kiosk files: on Day, with the work location Status,
// an optional deviation Reason and the Source every kiosk stamp carries, the
// card scan (SourceNFC).
type Stamp struct {
	StaffID int64
	Day     string
	Status  string
	Reason  string
	Source  string
}

// TimeClock is the consumer-owned port to the Workforce time clock. Every
// mutating operation is pinned to the calendar day the kiosk resolved, so a
// request that runs across midnight still acts on the block it was taken for.
// Refusals come back as StampError kinds or the typed refusals above; other
// failures pass through unchanged.
type TimeClock interface {
	CheckIn(ctx context.Context, stamp Stamp) (Session, error)
	CheckOutOn(ctx context.Context, staffID int64, day, reason string) (Session, error)
	StartBreakOn(ctx context.Context, staffID int64, day string, plannedDurationMinutes *int) error
	EndBreakOn(ctx context.Context, staffID int64, day string) (Session, error)
	// LatestOpenSession returns the block still running inside the live
	// window, whatever day it was opened on; found is false when there is
	// none.
	LatestOpenSession(ctx context.Context, staffID int64) (Session, bool, error)
	// Workday returns the day's blocks, breaks and labor-time figures as of
	// now.
	Workday(ctx context.Context, staffID int64, day string, now time.Time) (Workday, error)
}

// Clock supplies the instant a request is admitted and the calendar day an
// instant falls on in the school's local time.
type Clock interface {
	Now() time.Time
	Day(time.Time) string
}

// Dependencies are the collaborators the workflow cannot own. Every field is
// required so a missing wiring fails at startup.
type Dependencies struct {
	Cards     CardLookup
	Staff     StaffLookup
	TimeClock TimeClock
	Clock     Clock
}
