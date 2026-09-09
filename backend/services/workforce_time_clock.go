package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// timeClock serves workforce.TimeClock from the retained work session service
// while that service's own move into the Workforce module is pending (#2690).
type timeClock struct {
	sessions active.WorkSessionService
}

// TimeClockCapability binds the stamp contract of the web app and the kiosk
// to the work session service.
func TimeClockCapability(sessions active.WorkSessionService) workforce.TimeClock {
	if sessions == nil {
		panic("time clock capability: work session service is required")
	}
	return timeClock{sessions: sessions}
}

func (c timeClock) CheckIn(ctx context.Context, stamp workforce.CheckInStamp) (workforce.WorkSession, error) {
	var (
		session *activeModels.WorkSession
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
	return workSessionToCapability(session), nil
}

func (c timeClock) CheckOut(ctx context.Context, staffID int64, reason string) (workforce.WorkSession, error) {
	session, err := c.sessions.CheckOut(ctx, staffID, reason)
	if err != nil {
		return workforce.WorkSession{}, MapTimeTrackingError(err)
	}
	return workSessionToCapability(session), nil
}

func (c timeClock) StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (workforce.WorkSessionBreak, error) {
	workBreak, err := c.sessions.StartBreak(ctx, staffID, plannedDurationMinutes)
	if err != nil {
		return workforce.WorkSessionBreak{}, MapTimeTrackingError(err)
	}
	return workSessionBreakToCapability(workBreak), nil
}

func (c timeClock) EndBreak(ctx context.Context, staffID int64) (workforce.WorkSession, error) {
	session, err := c.sessions.EndBreak(ctx, staffID)
	if err != nil {
		return workforce.WorkSession{}, MapTimeTrackingError(err)
	}
	return workSessionToCapability(session), nil
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
	return workSessionToCapability(session), nil
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
	return workSessionBreakToCapability(workBreak), nil
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
	return workSessionToCapability(session), nil
}

func (c timeClock) LatestOpenSession(ctx context.Context, staffID int64) (workforce.WorkSession, bool, error) {
	session, err := c.sessions.GetLatestOpenSession(ctx, staffID)
	if err != nil {
		return workforce.WorkSession{}, false, MapTimeTrackingError(err)
	}
	if session == nil {
		return workforce.WorkSession{}, false, nil
	}
	return workSessionToCapability(session), true, nil
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
	sessions := make([]*activeModels.WorkSession, 0, len(history.Sessions))
	breaksBySession := make(map[int64][]*activeModels.WorkSessionBreak, len(history.Sessions))
	for _, daySession := range history.Sessions {
		if daySession == nil || daySession.WorkSession == nil {
			continue
		}
		session := daySession.WorkSession
		if expired, stale := active.ExpireStaleOpenBlock(session, now); stale {
			if !expired.CheckOutTime.After(dayStart) {
				continue
			}
			session = expired
		}
		sessions = append(sessions, session)
		breaksBySession[daySession.ID] = daySession.Breaks
		result.Sessions = append(result.Sessions, workSessionToCapability(session))
		breaks := make([]workforce.WorkSessionBreak, 0, len(daySession.Breaks))
		for _, workBreak := range daySession.Breaks {
			if workBreak != nil {
				breaks = append(breaks, workSessionBreakToCapability(workBreak))
			}
		}
		result.Breaks[daySession.ID] = breaks
	}
	evaluation := active.EvaluateWorkSessionsLaborTime(sessions, breaksBySession, now)
	result.LaborTime = workforce.LaborTimeEvaluation{
		NetMinutes: evaluation.NetMinutes, BreakMinutes: evaluation.BreakMinutes,
		RequiredBreakMinutes: evaluation.RequiredBreakMinutes, IsBreakCompliant: evaluation.IsBreakCompliant,
	}
	return result, nil
}

func timeClockDay(day string) (timezone.Date, error) {
	date, err := timezone.ParseDate(day)
	if err != nil {
		return timezone.Date(""), &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingInvalid, Cause: errors.New("day must be YYYY-MM-DD")}
	}
	return date, nil
}

// classifyCheckInError adds the raced-insert classification to the shared
// mapping. A concurrent stamp of the same card is a state conflict, not a
// server fault; the duplicate INSERT left the request transaction aborted, so
// the rollback is requested explicitly. The Workforce store reports the
// violated open-block index (#2402) as ErrWorkSessionAlreadyOpen.
func classifyCheckInError(ctx context.Context, err error) error {
	if errors.Is(err, workforce.ErrWorkSessionAlreadyOpen) {
		tenant.MarkRollback(ctx)
		return &workforce.TimeTrackingError{Kind: workforce.ErrCheckInRaced, Cause: err}
	}
	return MapTimeTrackingError(err)
}

// MapTimeTrackingError classifies the retained work session service errors
// into the public Workforce kinds. The wording of the cause is preserved: it is
// what the HTTP resources render. Errors outside the known set pass through
// untouched so 500 paths keep their cause.
func MapTimeTrackingError(err error) error {
	if err == nil {
		return nil
	}
	if plannedStart, ok := errors.AsType[*active.PlannedStartNotReachedError](err); ok {
		return &workforce.PlannedStartNotReachedError{PlannedStartTime: plannedStart.PlannedStartTime, CurrentTime: plannedStart.CurrentTime}
	}
	if deviation, ok := errors.AsType[*active.DeviationReasonRequiredError](err); ok {
		return &workforce.DeviationReasonRequiredError{
			Action: deviation.Action, PlannedTime: deviation.PlannedTime, ActualTime: deviation.ActualTime, DeviationMinutes: deviation.DeviationMinutes,
		}
	}
	msg := err.Error()
	switch {
	case msg == "already checked in", msg == "already checked out today", msg == "break already active":
		return &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingConflict, Cause: err}
	case strings.HasPrefix(msg, "work session overlaps an existing block"):
		return &workforce.TimeTrackingError{Kind: workforce.ErrWorkSessionOverlap, Cause: err}
	case msg == "no active session found", msg == "no session found for today", msg == "session not found", msg == "no active break found":
		return &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingNotFound, Cause: err}
	case msg == "can only update own sessions", msg == "session does not belong to requesting staff":
		return &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingForbidden, Cause: err}
	case strings.HasPrefix(msg, "status must be"),
		strings.HasPrefix(msg, "source must be"),
		msg == "notes required when changing status",
		msg == "notes required when changing recorded times",
		msg == "break minutes cannot be negative",
		msg == "break duration cannot be negative",
		msg == "invalid session data: check-in time must be before check-out time",
		msg == "check_out_time must be after check_in_time",
		strings.HasPrefix(msg, "planned_duration_minutes must be"),
		strings.HasPrefix(msg, "break ") && strings.Contains(msg, "does not belong to this session"),
		msg == "cannot edit duration of an active break":
		return &workforce.TimeTrackingError{Kind: workforce.ErrTimeTrackingInvalid, Cause: err}
	default:
		return err
	}
}

func workSessionToCapability(session *activeModels.WorkSession) workforce.WorkSession {
	if session == nil {
		return workforce.WorkSession{}
	}
	return workforce.WorkSession{
		ID: session.ID, TenantID: session.TenantID, StaffID: session.StaffID, Date: session.Date.String(),
		Status: session.Status, Source: session.Source, CheckInTime: session.CheckInTime, CheckOutTime: session.CheckOutTime,
		ReopenedAt: session.ReopenedAt, BreakMinutes: session.BreakMinutes, Notes: session.Notes,
		AutoCheckedOut: session.AutoCheckedOut, CreatedBy: session.CreatedBy, UpdatedBy: session.UpdatedBy,
		CreatedAt: session.CreatedAt, UpdatedAt: session.UpdatedAt,
	}
}

func workSessionBreakToCapability(workBreak *activeModels.WorkSessionBreak) workforce.WorkSessionBreak {
	if workBreak == nil {
		return workforce.WorkSessionBreak{}
	}
	return workforce.WorkSessionBreak{
		ID: workBreak.ID, TenantID: workBreak.TenantID, SessionID: workBreak.SessionID, StartedAt: workBreak.StartedAt,
		EndedAt: workBreak.EndedAt, DurationMinutes: workBreak.DurationMinutes, PlannedEndTime: workBreak.PlannedEndTime,
		CreatedAt: workBreak.CreatedAt, UpdatedAt: workBreak.UpdatedAt,
	}
}
