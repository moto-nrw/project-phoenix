package services

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/iot/staffclock"
)

// staffClockTimeClock serves the kiosk workflow's TimeClock port from the
// public Workforce time clock (#2690). Every stamp the kiosk files carries the
// NFC source; refusals are translated into the kiosk error kinds.
type staffClockTimeClock struct {
	clock workforce.TimeClock
}

// StaffClockTimeClock adapts the public Workforce time clock to the staff
// clock workflow's port.
func StaffClockTimeClock(clock workforce.TimeClock) staffclock.TimeClock {
	if clock == nil {
		panic("staff clock time clock: workforce time clock is required")
	}
	return staffClockTimeClock{clock: clock}
}

func (c staffClockTimeClock) CheckIn(ctx context.Context, stamp staffclock.Stamp) (staffclock.Session, error) {
	session, err := c.clock.CheckIn(ctx, workforce.CheckInStamp{
		StaffID: stamp.StaffID, Day: stamp.Day, Status: stamp.Status, Source: workforce.WorkSessionSourceNFC, Reason: stamp.Reason,
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

func kioskSession(session workforce.WorkSession) staffclock.Session {
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
	if plannedStart, ok := errors.AsType[*workforce.PlannedStartNotReachedError](err); ok {
		return &staffclock.PlannedStartNotReachedError{PlannedStartTime: plannedStart.PlannedStartTime, CurrentTime: plannedStart.CurrentTime}
	}
	if deviation, ok := errors.AsType[*workforce.DeviationReasonRequiredError](err); ok {
		return &staffclock.DeviationReasonRequiredError{
			Action: deviation.Action, PlannedTime: deviation.PlannedTime, ActualTime: deviation.ActualTime, DeviationMinutes: deviation.DeviationMinutes,
		}
	}
	switch {
	case errors.Is(err, workforce.ErrCheckInRaced):
		return &staffclock.StampError{Kind: staffclock.ErrCheckInRaced, Cause: err}
	case errors.Is(err, workforce.ErrTimeTrackingConflict), errors.Is(err, workforce.ErrTimeTrackingNotFound), errors.Is(err, workforce.ErrWorkSessionOverlap):
		return &staffclock.StampError{Kind: staffclock.ErrStateConflict, Cause: err}
	case errors.Is(err, workforce.ErrTimeTrackingInvalid):
		return &staffclock.StampError{Kind: staffclock.ErrInvalidStamp, Cause: err}
	default:
		return err
	}
}
