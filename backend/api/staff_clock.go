package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	clockCompose "github.com/moto-nrw/project-phoenix/workflows/staffclock/compose"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	staffclock "github.com/moto-nrw/project-phoenix/workflows/staffclock"
)

// The staff-clock workflow (the kiosk's check-in and check-out) is bound here
// over the card owner (Identity & Access), the person lookup (People
// Directory) and the Workforce time clock.

// newStaffClockService composes the staff-clock workflow over the retained
// person service, the card owner's capability and the Workforce time clock.
func newStaffClockService(people peoplePorts.PersonDirectory, cards identityaccess.RFIDCards, clock workforceModule.TimeClock) devicescan.StaffClock {
	return clockCompose.NewService(staffclock.Dependencies{
		Cards:     staffClockCards{cards: cards},
		Staff:     newStaffClockStaffLookup(people),
		TimeClock: newStaffClockTimeClock(clock),
		Clock:     staffClockClock{},
	})
}

// staffClockClock is the wall clock and the Berlin calendar day the kiosk
// workflow reasons with.
type staffClockClock struct{}

func (staffClockClock) Now() time.Time               { return time.Now() }
func (staffClockClock) Day(instant time.Time) string { return timezone.DateFromTime(instant).String() }

// staffClockCards maps the owner's validated card facts to the staff-clock port.
type staffClockCards struct{ cards identityaccess.RFIDCards }

func (l staffClockCards) FindCard(ctx context.Context, rawTag string) (*staffclock.Card, error) {
	if err := l.cards.ValidateRFIDTag(rawTag); err != nil {
		return nil, fmt.Errorf("%w: %v", staffclock.ErrInvalidRFIDTag, err)
	}
	id, active, found, err := l.cards.LookupRFIDCard(ctx, rawTag)
	if err != nil || !found {
		return nil, err
	}
	return &staffclock.Card{ID: id, Active: active}, nil
}

// staffClockStaffLookup resolves the staff member behind a normalized tag
// through the retained person service. It answers the two stable kiosk
// outcomes the workflow classifies on and lets every other failure through.
type staffClockStaffLookup struct {
	people peoplePorts.PersonDirectory
}

func newStaffClockStaffLookup(people peoplePorts.PersonDirectory) staffclock.StaffLookup {
	if people == nil {
		panic("staff clock staff lookup: person service is required")
	}
	return staffClockStaffLookup{people: people}
}

func (l staffClockStaffLookup) ResolveStaffByTag(ctx context.Context, tag string) (staffclock.StaffIdentity, error) {
	person, err := l.people.FindByTagID(ctx, tag)
	if err != nil {
		if peoplePorts.IsPersonNotFound(err) {
			return staffclock.StaffIdentity{}, staffclock.ErrRFIDTagNotFound
		}
		return staffclock.StaffIdentity{}, fmt.Errorf("look up person by RFID tag: %w", err)
	}
	// An active card that is not linked to anybody resolves to no person. The
	// kiosk gets the same stable "unknown tag" answer it gets for a card the
	// system has never seen.
	if person == nil {
		return staffclock.StaffIdentity{}, staffclock.ErrRFIDTagNotFound
	}
	staff, err := l.people.GetStaffByPersonID(ctx, person.ID)
	if err != nil {
		if errors.Is(err, transactionruntime.ErrNoRows) {
			return staffclock.StaffIdentity{}, staffclock.ErrRFIDTagNotStaff
		}
		return staffclock.StaffIdentity{}, err
	}
	if staff == nil {
		return staffclock.StaffIdentity{}, staffclock.ErrRFIDTagNotStaff
	}
	return staffclock.StaffIdentity{StaffID: staff.ID, FullName: person.GetFullName()}, nil
}

// staffClockTimeClock serves the kiosk workflow's TimeClock port from the
// public Workforce time clock (#2690). Refusals are translated into the kiosk
// error kinds.
type staffClockTimeClock struct {
	clock workforceModule.TimeClock
}

func newStaffClockTimeClock(clock workforceModule.TimeClock) staffclock.TimeClock {
	if clock == nil {
		panic("staff clock time clock: workforce time clock is required")
	}
	return staffClockTimeClock{clock: clock}
}

func (c staffClockTimeClock) CheckIn(ctx context.Context, stamp staffclock.Stamp) (staffclock.Session, error) {
	session, err := c.clock.CheckIn(ctx, workforceModule.CheckInStamp{
		StaffID: stamp.StaffID, Day: stamp.Day, Status: stamp.Status, Source: stamp.Source, Reason: stamp.Reason,
	})
	if err != nil {
		return staffclock.Session{}, mapStampError(err)
	}
	return kioskSession(session), nil
}

func (c staffClockTimeClock) CheckOutOn(ctx context.Context, staffID int64, day, reason string) (staffclock.Session, error) {
	session, err := c.clock.CheckOutOn(ctx, staffID, day, reason)
	if err != nil {
		return staffclock.Session{}, mapStampError(err)
	}
	return kioskSession(session), nil
}

func (c staffClockTimeClock) StartBreakOn(ctx context.Context, staffID int64, day string, plannedDurationMinutes *int) error {
	if _, err := c.clock.StartBreakOn(ctx, staffID, day, plannedDurationMinutes); err != nil {
		return mapStampError(err)
	}
	return nil
}

func (c staffClockTimeClock) EndBreakOn(ctx context.Context, staffID int64, day string) (staffclock.Session, error) {
	session, err := c.clock.EndBreakOn(ctx, staffID, day)
	if err != nil {
		return staffclock.Session{}, mapStampError(err)
	}
	return kioskSession(session), nil
}

func (c staffClockTimeClock) LatestOpenSession(ctx context.Context, staffID int64) (staffclock.Session, bool, error) {
	session, found, err := c.clock.LatestOpenSession(ctx, staffID)
	if err != nil || !found {
		return staffclock.Session{}, false, err
	}
	return kioskSession(session), true, nil
}

func (c staffClockTimeClock) Workday(ctx context.Context, staffID int64, day string, now time.Time) (staffclock.Workday, error) {
	workday, err := c.clock.Workday(ctx, staffID, day, now)
	if err != nil {
		return staffclock.Workday{}, err
	}
	result := staffclock.Workday{
		Sessions:             make([]staffclock.Session, 0, len(workday.Sessions)),
		Breaks:               make(map[int64][]staffclock.Break, len(workday.Breaks)),
		NetMinutes:           workday.LaborTime.NetMinutes,
		BreakMinutes:         workday.LaborTime.BreakMinutes,
		RequiredBreakMinutes: workday.LaborTime.RequiredBreakMinutes,
		IsBreakCompliant:     workday.LaborTime.IsBreakCompliant,
	}
	for _, session := range workday.Sessions {
		result.Sessions = append(result.Sessions, kioskSession(session))
	}
	for sessionID, breaks := range workday.Breaks {
		kioskBreaks := make([]staffclock.Break, 0, len(breaks))
		for _, workBreak := range breaks {
			kioskBreaks = append(kioskBreaks, staffclock.Break{ID: workBreak.ID, StartedAt: workBreak.StartedAt, EndedAt: workBreak.EndedAt})
		}
		result.Breaks[sessionID] = kioskBreaks
	}
	return result, nil
}

func kioskSession(session workforceModule.WorkSession) staffclock.Session {
	return staffclock.Session{
		ID: session.ID, StaffID: session.StaffID, Day: session.Date, CheckInTime: session.CheckInTime,
		CheckOutTime: session.CheckOutTime, Status: session.Status, Source: session.Source,
	}
}

// mapStampError translates the Workforce time clock refusals into the kiosk
// contract. A state conflict and a missing block are the same thing at a
// hallway terminal: rescan for authoritative state. Unclassified failures pass
// through so they keep their 500 classification.
func mapStampError(err error) error {
	if plannedStart, ok := errors.AsType[*workforceModule.PlannedStartNotReachedError](err); ok {
		return &staffclock.PlannedStartNotReachedError{PlannedStartTime: plannedStart.PlannedStartTime, CurrentTime: plannedStart.CurrentTime}
	}
	if deviation, ok := errors.AsType[*workforceModule.DeviationReasonRequiredError](err); ok {
		return &staffclock.DeviationReasonRequiredError{
			Action: deviation.Action, PlannedTime: deviation.PlannedTime, ActualTime: deviation.ActualTime, DeviationMinutes: deviation.DeviationMinutes,
		}
	}
	switch {
	case errors.Is(err, workforceModule.ErrCheckInRaced):
		return &staffclock.StampError{Kind: staffclock.ErrCheckInRaced, Cause: err}
	case errors.Is(err, workforceModule.ErrTimeTrackingConflict), errors.Is(err, workforceModule.ErrTimeTrackingNotFound), errors.Is(err, workforceModule.ErrWorkSessionOverlap):
		return &staffclock.StampError{Kind: staffclock.ErrStateConflict, Cause: err}
	case errors.Is(err, workforceModule.ErrTimeTrackingInvalid):
		return &staffclock.StampError{Kind: staffclock.ErrInvalidStamp, Cause: err}
	default:
		return err
	}
}
