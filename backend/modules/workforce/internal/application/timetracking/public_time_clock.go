package timetracking

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// timeClock serves workforce.TimeClock from the work session service.
type timeClock struct {
	sessions WorkSessionService
}

// TimeClockCapability binds the stamp contract of the web app and the kiosk
// to the work session service.
func TimeClockCapability(sessions WorkSessionService) workforce.TimeClock {
	if sessions == nil {
		panic("time clock capability: work session service is required")
	}
	return timeClock{sessions: sessions}
}

func (c timeClock) CheckIn(ctx context.Context, stamp workforce.CheckInStamp) (workforce.WorkSession, error) {
	var (
		session *WorkSession
		err     error
	)
	if stamp.Day == "" {
		session, err = c.sessions.CheckIn(ctx, stamp.StaffID, stamp.Status, stamp.Source, stamp.Reason)
	} else {
		day, parseErr := timeClockDay(stamp.Day)
		if parseErr != nil {
			return workforce.WorkSession{}, parseErr
		}
		session, err = c.sessions.CheckInOn(ctx, stamp.StaffID, day, stamp.Status, stamp.Source, stamp.Reason)
	}
	if err != nil {
		return workforce.WorkSession{}, classifyCheckInError(ctx, err)
	}
	return WorkSessionToCapability(session), nil
}

func (c timeClock) CheckOut(ctx context.Context, staffID int64, reason string) (workforce.WorkSession, error) {
	session, err := c.sessions.CheckOut(ctx, staffID, reason)
	if err != nil {
		return workforce.WorkSession{}, MapTimeTrackingError(err)
	}
	return WorkSessionToCapability(session), nil
}

func (c timeClock) StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (workforce.WorkSessionBreak, error) {
	workBreak, err := c.sessions.StartBreak(ctx, staffID, plannedDurationMinutes)
	if err != nil {
		return workforce.WorkSessionBreak{}, MapTimeTrackingError(err)
	}
	return WorkSessionBreakToCapability(workBreak), nil
}

func (c timeClock) EndBreak(ctx context.Context, staffID int64) (workforce.WorkSession, error) {
	session, err := c.sessions.EndBreak(ctx, staffID)
	if err != nil {
		return workforce.WorkSession{}, MapTimeTrackingError(err)
	}
	return WorkSessionToCapability(session), nil
}

func (c timeClock) CheckOutOn(ctx context.Context, staffID int64, day, reason string) (workforce.WorkSession, error) {
	date, err := timeClockDay(day)
	if err != nil {
		return workforce.WorkSession{}, err
	}
	session, err := c.sessions.CheckOutOn(ctx, staffID, date, reason)
	if err != nil {
		return workforce.WorkSession{}, MapTimeTrackingError(err)
	}
	return WorkSessionToCapability(session), nil
}

func (c timeClock) StartBreakOn(ctx context.Context, staffID int64, day string, plannedDurationMinutes *int) (workforce.WorkSessionBreak, error) {
	date, err := timeClockDay(day)
	if err != nil {
		return workforce.WorkSessionBreak{}, err
	}
	workBreak, err := c.sessions.StartBreakOn(ctx, staffID, date, plannedDurationMinutes)
	if err != nil {
		return workforce.WorkSessionBreak{}, MapTimeTrackingError(err)
	}
	return WorkSessionBreakToCapability(workBreak), nil
}

func (c timeClock) EndBreakOn(ctx context.Context, staffID int64, day string) (workforce.WorkSession, error) {
	date, err := timeClockDay(day)
	if err != nil {
		return workforce.WorkSession{}, err
	}
	session, err := c.sessions.EndBreakOn(ctx, staffID, date)
	if err != nil {
		return workforce.WorkSession{}, MapTimeTrackingError(err)
	}
	return WorkSessionToCapability(session), nil
}

func (c timeClock) LatestOpenSession(ctx context.Context, staffID int64) (workforce.WorkSession, bool, error) {
	session, err := c.sessions.GetLatestOpenSession(ctx, staffID)
	if err != nil {
		return workforce.WorkSession{}, false, MapTimeTrackingError(err)
	}
	if session == nil {
		return workforce.WorkSession{}, false, nil
	}
	return WorkSessionToCapability(session), true, nil
}

// Workday reads the blocks intersecting the day. An open block that passed its
// live limit is cut at the instant it stopped counting as work everywhere else
// (ExpireStaleOpenBlock) and drops out entirely once that instant lies before
// the day begins; the stored row is repaired on the next check-in, not here.
func (c timeClock) Workday(ctx context.Context, staffID int64, day string, now time.Time) (workforce.Workday, error) {
	date, err := timeClockDay(day)
	if err != nil {
		return workforce.Workday{}, err
	}
	history, err := c.sessions.GetHistoryIntersecting(ctx, staffID, date, date)
	if err != nil {
		return workforce.Workday{}, MapTimeTrackingError(err)
	}
	result := workforce.Workday{Sessions: []workforce.WorkSession{}, Breaks: map[int64][]workforce.WorkSessionBreak{}}
	if history == nil || len(history.Sessions) == 0 {
		return result, nil
	}
	dayStart := date.BerlinMidnight()
	sessions := make([]*WorkSession, 0, len(history.Sessions))
	breaksBySession := make(map[int64][]*WorkSessionBreak, len(history.Sessions))
	for _, daySession := range history.Sessions {
		if daySession == nil || daySession.WorkSession == nil {
			continue
		}
		session, counts := workdaySession(daySession.WorkSession, dayStart, now)
		if !counts {
			continue
		}
		sessions = append(sessions, session)
		breaksBySession[daySession.ID] = daySession.Breaks
		result.Sessions = append(result.Sessions, WorkSessionToCapability(session))
		result.Breaks[daySession.ID] = workdayBreaks(daySession.Breaks)
	}
	evaluation := EvaluateWorkSessionsLaborTime(sessions, breaksBySession, now)
	result.LaborTime = workforce.LaborTimeEvaluation{
		NetMinutes: evaluation.NetMinutes, BreakMinutes: evaluation.BreakMinutes,
		RequiredBreakMinutes: evaluation.RequiredBreakMinutes, IsBreakCompliant: evaluation.IsBreakCompliant,
	}
	return result, nil
}

// workdaySession is the block as the day sees it: an open block past its live
// limit counts as closed at that limit and not at all once the limit lies
// before the day begins.
func workdaySession(session *WorkSession, dayStart, now time.Time) (*WorkSession, bool) {
	expired, stale := ExpireStaleOpenBlock(session, now)
	if !stale {
		return session, true
	}
	return expired, expired.CheckOutTime.After(dayStart)
}

func workdayBreaks(values []*WorkSessionBreak) []workforce.WorkSessionBreak {
	breaks := make([]workforce.WorkSessionBreak, 0, len(values))
	for _, workBreak := range values {
		if workBreak != nil {
			breaks = append(breaks, WorkSessionBreakToCapability(workBreak))
		}
	}
	return breaks
}

func timeClockDay(day string) (calendar.Date, error) {
	date, err := calendar.ParseDate(day)
	if err != nil {
		return calendar.Date(""), &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingInvalid, Cause: errors.New("day must be YYYY-MM-DD")}
	}
	return date, nil
}

// classifyCheckInError adds the raced-insert classification to the shared
// mapping.
func classifyCheckInError(ctx context.Context, err error) error {
	if raced := racedCheckIn(ctx, err); raced != nil {
		return raced
	}
	return MapTimeTrackingError(err)
}

// racedCheckIn classifies a check-in that lost the race for the open block.
// A concurrent stamp of the same card is a state conflict, not a server
// fault; the duplicate INSERT left the request transaction aborted, so the
// rollback is requested explicitly. The Workforce store reports the violated
// open-block index (#2402) as ErrWorkSessionAlreadyOpen. Any other error is
// not a race and yields nil.
func racedCheckIn(ctx context.Context, err error) error {
	if !errors.Is(err, workforce.ErrWorkSessionAlreadyOpen) {
		return nil
	}
	tenant.MarkRollback(ctx)
	return &workforce.TimeTrackingError{Kind: workforce.ErrCheckInRaced, Cause: err}
}

// MapTimeTrackingError classifies the retained work session service errors
// into the public Workforce kinds. The wording of the cause is preserved: it is
// what the HTTP resources render. Errors outside the known set pass through
// untouched so 500 paths keep their cause.
func MapTimeTrackingError(err error) error {
	if err == nil {
		return nil
	}
	if plannedStart, ok := errors.AsType[*PlannedStartNotReachedError](err); ok {
		return &workforce.PlannedStartNotReachedError{PlannedStartTime: plannedStart.PlannedStartTime, CurrentTime: plannedStart.CurrentTime}
	}
	if deviation, ok := errors.AsType[*DeviationReasonRequiredError](err); ok {
		return &workforce.DeviationReasonRequiredError{
			Action: deviation.Action, PlannedTime: deviation.PlannedTime, ActualTime: deviation.ActualTime, DeviationMinutes: deviation.DeviationMinutes,
		}
	}
	switch {
	case matchesTimeTrackingError(err, workforce.ErrAlreadyCheckedIn, workforce.ErrAlreadyCheckedOut, workforce.ErrBreakAlreadyActive):
		return &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingConflict, Cause: err}
	case errors.Is(err, workforce.ErrWorkSessionOverlap):
		return err
	case matchesTimeTrackingError(err, workforce.ErrNoActiveSession, workforce.ErrNoSessionForToday, workforce.ErrSessionNotFound, workforce.ErrNoActiveBreak):
		return &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingNotFound, Cause: err}
	case matchesTimeTrackingError(err, workforce.ErrSessionUpdateNotOwned, workforce.ErrSessionNotOwned):
		return &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingForbidden, Cause: err}
	case matchesTimeTrackingError(err, workforce.ErrSessionStatusInvalid, workforce.ErrSessionSourceInvalid,
		workforce.ErrSessionStatusNoteRequired, workforce.ErrSessionTimesNoteRequired, workforce.ErrBreakMinutesNegative,
		workforce.ErrBreakDurationNegative, workforce.ErrSessionTimeOrder, workforce.ErrSessionCheckoutOrder,
		workforce.ErrPlannedBreakDurationInvalid, workforce.ErrBreakNotOwned, workforce.ErrActiveBreakDurationEdit):
		return &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingInvalid, Cause: err}
	default:
		return err
	}
}

func matchesTimeTrackingError(err error, targets ...error) bool {
	for _, target := range targets {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
