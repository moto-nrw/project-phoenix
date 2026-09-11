// Package staffclock coordinates NFC-based staff time tracking for kiosks. It
// is the staff-clock workflow: it resolves the scanned card to a staff member
// and stamps through consumer-owned ports, then renders the kiosk state the
// device-scan contract promises. The composition root adapts the public
// Workforce time clock to the TimeClock port.
package staffclock

import (
	"context"
	"errors"
	"fmt"
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

func (e *StampError) Error() string        { return e.Cause.Error() }
func (e *StampError) Is(target error) bool { return target == e.Kind }
func (e *StampError) Unwrap() error        { return e.Cause }

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

// Stamp is a check-in the kiosk files: on Day, with the work location Status
// and an optional deviation Reason.
type Stamp struct {
	StaffID int64
	Day     string
	Status  string
	Reason  string
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

// Service owns the pure NFC stamp workflow. It deliberately accepts narrow
// ports so its state machine can be tested without HTTP or a database.
type Service struct {
	cards     CardLookup
	staff     StaffLookup
	timeClock TimeClock
	clock     Clock
}

var _ devicescan.StaffClock = (*Service)(nil)

func NewService(dependencies Dependencies) *Service {
	if dependencies.Cards == nil || dependencies.Staff == nil || dependencies.TimeClock == nil || dependencies.Clock == nil {
		panic("staff clock: all dependencies are required")
	}
	return &Service{cards: dependencies.Cards, staff: dependencies.Staff, timeClock: dependencies.TimeClock, clock: dependencies.Clock}
}

// StaffClockState resolves a staff card and returns the authoritative kiosk
// state.
func (s *Service) StaffClockState(ctx context.Context, rawTag string) (*devicescan.StaffClockState, error) {
	staff, err := s.resolveStaff(ctx, rawTag)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	day, err := s.clockDay(ctx, staff.StaffID, now)
	if err != nil {
		return nil, err
	}
	return s.loadState(ctx, staff, day, now)
}

// clockDay is the calendar day one kiosk request works on: the day carried by
// the session that is still running, or today when nobody is clocked in.
//
// Taking "today" unconditionally would hide a session opened before Berlin
// midnight. The kiosk would report checked_out to somebody who is demonstrably
// still at work, offer a second check-in on the new day, and leave the first
// session with no way to be closed through this flow at all.
func (s *Service) clockDay(ctx context.Context, staffID int64, now time.Time) (string, error) {
	open, found, err := s.timeClock.LatestOpenSession(ctx, staffID)
	if err != nil {
		return "", fmt.Errorf("look up running work session: %w", err)
	}
	if found {
		return open.Day, nil
	}
	return s.clock.Day(now), nil
}

// ExecuteStaffClock performs one stamp and returns the resulting kiosk state.
// The day is the running session's own day when there is one (a night shift
// keeps working on the day it started), otherwise today.
func (s *Service) ExecuteStaffClock(ctx context.Context, command devicescan.StaffClockCommand) (*devicescan.StaffClockState, error) {
	staff, err := s.resolveStaff(ctx, command.RFIDTag)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	day, err := s.clockDay(ctx, staff.StaffID, now)
	if err != nil {
		return nil, err
	}

	var stamped Session
	stampedDay := false
	switch command.Action {
	case ActionCheckIn:
		stamped, err = s.checkIn(ctx, staff.StaffID, day, command)
		stampedDay = err == nil
	case ActionCheckOut:
		stamped, err = s.timeClock.CheckOutOn(ctx, staff.StaffID, day, command.Reason)
		stampedDay = err == nil
	case ActionBreakStart:
		err = s.timeClock.StartBreakOn(ctx, staff.StaffID, day, command.PlannedDurationMinutes)
	case ActionBreakEnd:
		stamped, err = s.timeClock.EndBreakOn(ctx, staff.StaffID, day)
		stampedDay = err == nil
	default:
		return nil, ErrInvalidAction
	}
	if err != nil {
		return nil, err
	}
	if stampedDay && stamped.Day != "" {
		day = stamped.Day
	}

	// The labor-time figures are read against the clock as it stands after the
	// write, not against the instant the request was admitted. `now` selected
	// the day and is deliberately left alone; reusing it here would measure the
	// session from before its own check-in whenever the stamp crossed midnight,
	// reporting zero elapsed work and a break requirement computed for the
	// wrong point in the shift.
	return s.loadState(ctx, staff, day, s.clock.Now())
}

// checkIn stamps the arrival. Since #2402 a repeated check-in after a
// checkout creates a NEW work block with its own status, so a Homeoffice
// morning followed by an OGS afternoon needs no reason and no follow-up edit.
// The stamped session is returned so the caller can read the state back on
// the day the row actually carries.
func (s *Service) checkIn(ctx context.Context, staffID int64, day string, command devicescan.StaffClockCommand) (Session, error) {
	if command.Status == "" {
		return Session{}, ErrStatusRequired
	}
	if command.Status != StatusPresent && command.Status != StatusHomeOffice {
		return Session{}, fmt.Errorf("%w: status must be 'present' or 'home_office'", ErrStatusRequired)
	}
	return s.timeClock.CheckIn(ctx, Stamp{StaffID: staffID, Day: day, Status: command.Status, Reason: command.Reason})
}

func (s *Service) resolveStaff(ctx context.Context, rawTag string) (StaffIdentity, error) {
	card, err := s.cards.FindCard(ctx, rawTag)
	if err != nil {
		if errors.Is(err, ErrInvalidRFIDTag) {
			return StaffIdentity{}, err
		}
		return StaffIdentity{}, fmt.Errorf("look up RFID card: %w", err)
	}
	if card == nil {
		return StaffIdentity{}, ErrRFIDTagNotFound
	}
	if !card.Active {
		return StaffIdentity{}, ErrRFIDTagInactive
	}
	staff, err := s.staff.ResolveStaffByTag(ctx, card.ID)
	if err != nil {
		if errors.Is(err, ErrRFIDTagNotFound) || errors.Is(err, ErrRFIDTagNotStaff) {
			return StaffIdentity{}, err
		}
		return StaffIdentity{}, fmt.Errorf("look up staff for RFID tag: %w", err)
	}
	if staff.StaffID <= 0 {
		return StaffIdentity{}, ErrRFIDTagNotStaff
	}
	return staff, nil
}

// loadState renders the kiosk state of `day`. It never derives that day
// itself: the caller passes the day the session was written on, while `now`
// is only the reference instant for the labor-time figures.
func (s *Service) loadState(ctx context.Context, staff StaffIdentity, day string, now time.Time) (*devicescan.StaffClockState, error) {
	result := &devicescan.StaffClockState{
		StaffID:          staff.StaffID,
		StaffName:        staff.FullName,
		State:            StateCheckedOut,
		AllowedActions:   []string{ActionCheckIn},
		IsBreakCompliant: true,
	}

	workday, err := s.timeClock.Workday(ctx, staff.StaffID, day, now)
	if err != nil {
		return nil, fmt.Errorf("load work sessions of the day: %w", err)
	}
	if len(workday.Sessions) == 0 {
		return result, nil
	}

	// The block the kiosk acts on: the open one when someone is clocked in,
	// otherwise the chronologically last block of the day (checked-out
	// summary + the offer to start a new block).
	current := workday.Sessions[len(workday.Sessions)-1]
	for _, session := range workday.Sessions {
		if session.IsOpen() {
			current = session
			break
		}
	}

	result.Session = &devicescan.StaffClockSession{
		ID:           current.ID,
		StaffID:      current.StaffID,
		CheckInTime:  current.CheckInTime,
		CheckOutTime: current.CheckOutTime,
		Status:       current.Status,
		Source:       current.Source,
	}
	if current.IsOpen() {
		result.State = StateCheckedIn
		result.AllowedActions = []string{ActionBreakStart, ActionCheckOut}
		for _, workBreak := range workday.Breaks[current.ID] {
			if workBreak.IsActive() {
				result.State = StateOnBreak
				result.ActiveBreak = &devicescan.StaffClockBreak{ID: workBreak.ID, StartedAt: workBreak.StartedAt}
				result.AllowedActions = []string{ActionBreakEnd, ActionCheckOut}
				break
			}
		}
	}

	// The labor-time figures cover the complete workday, not just the current
	// block: with a Homeoffice morning and an OGS afternoon (#2402) the kiosk
	// must show the summed work time, and §4 ArbZG judges the day as a whole.
	result.NetMinutes = workday.NetMinutes
	result.BreakMinutes = workday.BreakMinutes
	result.RequiredBreakMinutes = workday.RequiredBreakMinutes
	result.IsBreakCompliant = workday.IsBreakCompliant
	return result, nil
}
