package staffclock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	clockcontract "github.com/moto-nrw/project-phoenix/workflows/staffclock"
)

// Service owns the pure NFC stamp workflow. It deliberately accepts narrow
// ports so its state machine can be tested without HTTP or a database.
type Service struct {
	cards     clockcontract.CardLookup
	staff     clockcontract.StaffLookup
	timeClock clockcontract.TimeClock
	clock     clockcontract.Clock
}

func NewService(dependencies clockcontract.Dependencies) *Service {
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
	open, found, err := s.timeClock.LatestOpenSession(ctx, staff.StaffID)
	if err != nil {
		return nil, fmt.Errorf("look up running work session: %w", err)
	}
	var day string
	if found {
		day = open.Day
	} else {
		day = s.clock.Day(now)
	}
	return s.loadState(ctx, staff, day, now)
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
	open, found, err := s.timeClock.LatestOpenSession(ctx, staff.StaffID)
	if err != nil {
		return nil, fmt.Errorf("look up running work session: %w", err)
	}
	var day string
	if found {
		day = open.Day
	} else {
		day = s.clock.Day(now)
	}

	var stamped clockcontract.Session
	stampedDay := false
	switch command.Action {
	case clockcontract.ActionCheckIn:
		stamped, err = s.checkIn(ctx, staff.StaffID, day, command)
		stampedDay = err == nil
	case clockcontract.ActionCheckOut:
		stamped, err = s.timeClock.CheckOutOn(ctx, staff.StaffID, day, command.Reason)
		stampedDay = err == nil
	case clockcontract.ActionBreakStart:
		err = s.timeClock.StartBreakOn(ctx, staff.StaffID, day, command.PlannedDurationMinutes)
	case clockcontract.ActionBreakEnd:
		stamped, err = s.timeClock.EndBreakOn(ctx, staff.StaffID, day)
		stampedDay = err == nil
	default:
		return nil, clockcontract.ErrInvalidAction
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
func (s *Service) checkIn(ctx context.Context, staffID int64, day string, command devicescan.StaffClockCommand) (clockcontract.Session, error) {
	if command.Status == "" {
		return clockcontract.Session{}, clockcontract.ErrStatusRequired
	}
	if command.Status != clockcontract.StatusPresent && command.Status != clockcontract.StatusHomeOffice {
		return clockcontract.Session{}, fmt.Errorf("%w: status must be 'present' or 'home_office'", clockcontract.ErrStatusRequired)
	}
	return s.timeClock.CheckIn(ctx, clockcontract.Stamp{StaffID: staffID, Day: day, Status: command.Status, Reason: command.Reason, Source: clockcontract.SourceNFC})
}

func (s *Service) resolveStaff(ctx context.Context, rawTag string) (clockcontract.StaffIdentity, error) {
	card, err := s.cards.FindCard(ctx, rawTag)
	if err != nil {
		if errors.Is(err, clockcontract.ErrInvalidRFIDTag) {
			return clockcontract.StaffIdentity{}, err
		}
		return clockcontract.StaffIdentity{}, fmt.Errorf("look up RFID card: %w", err)
	}
	if card == nil {
		return clockcontract.StaffIdentity{}, clockcontract.ErrRFIDTagNotFound
	}
	if !card.Active {
		return clockcontract.StaffIdentity{}, clockcontract.ErrRFIDTagInactive
	}
	staff, err := s.staff.ResolveStaffByTag(ctx, card.ID)
	if err != nil {
		if errors.Is(err, clockcontract.ErrRFIDTagNotFound) || errors.Is(err, clockcontract.ErrRFIDTagNotStaff) {
			return clockcontract.StaffIdentity{}, err
		}
		return clockcontract.StaffIdentity{}, fmt.Errorf("look up staff for RFID tag: %w", err)
	}
	if staff.StaffID <= 0 {
		return clockcontract.StaffIdentity{}, clockcontract.ErrRFIDTagNotStaff
	}
	return staff, nil
}

// loadState renders the kiosk state of `day`. It never derives that day
// itself: the caller passes the day the session was written on, while `now`
// is only the reference instant for the labor-time figures.
func (s *Service) loadState(ctx context.Context, staff clockcontract.StaffIdentity, day string, now time.Time) (*devicescan.StaffClockState, error) {
	result := &devicescan.StaffClockState{
		StaffID:          staff.StaffID,
		StaffName:        staff.FullName,
		State:            clockcontract.StateCheckedOut,
		AllowedActions:   []string{clockcontract.ActionCheckIn},
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
		result.State = clockcontract.StateCheckedIn
		result.AllowedActions = []string{clockcontract.ActionBreakStart, clockcontract.ActionCheckOut}
		for _, workBreak := range workday.Breaks[current.ID] {
			if workBreak.IsActive() {
				result.State = clockcontract.StateOnBreak
				result.ActiveBreak = &devicescan.StaffClockBreak{ID: workBreak.ID, StartedAt: workBreak.StartedAt}
				result.AllowedActions = []string{clockcontract.ActionBreakEnd, clockcontract.ActionCheckOut}
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

var _ devicescan.StaffClock = (*Service)(nil)
