package timetracking

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// StartBreak starts a new break on the running block, whichever day it was
// opened on (see openBlockDay).
// If plannedDurationMinutes is provided (1-240), sets planned_end_time for auto-end
func (s *workSessionService) StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (*WorkSessionBreak, error) {
	day, err := s.openBlockDay(ctx, staffID)
	if err != nil {
		return nil, err
	}
	return s.StartBreakOn(ctx, staffID, day, plannedDurationMinutes)
}

// StartBreakOn starts a break on the open session of an explicit calendar day.
func (s *workSessionService) StartBreakOn(ctx context.Context, staffID int64, day calendar.Date, plannedDurationMinutes *int) (*WorkSessionBreak, error) {
	if err := s.lockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	// Get the open session of the requested day
	session, err := s.repo.GetOpenByStaffAndDateForUpdate(ctx, staffID, day)
	if err != nil {
		if isRecordNotFound(err) {
			return nil, workforce.ErrNoActiveSession
		}
		return nil, fmt.Errorf(errGetCurrentSession, err)
	}
	if session == nil {
		return nil, workforce.ErrNoActiveSession
	}

	// Check no active break exists
	activeBreak, err := s.breakRepo.GetActiveBySessionID(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check active break: %w", err)
	}
	if activeBreak != nil {
		return nil, workforce.ErrBreakAlreadyActive
	}

	// Validate plannedDurationMinutes if provided
	if plannedDurationMinutes != nil {
		if *plannedDurationMinutes < 1 || *plannedDurationMinutes > maxPlannedBreakDurationMinutes {
			return nil, &workforce.TimeTrackingError{Kind: workforce.ErrPlannedBreakDurationInvalid, Cause: fmt.Errorf("planned_duration_minutes must be between 1 and %d", maxPlannedBreakDurationMinutes)}
		}
	}

	// Create a new break
	now := s.now()
	brk := &WorkSessionBreak{
		SessionID: session.ID,
		StartedAt: now,
	}

	// Set planned_end_time if duration is specified
	if plannedDurationMinutes != nil {
		plannedEnd := now.Add(time.Duration(*plannedDurationMinutes) * time.Minute)
		brk.PlannedEndTime = &plannedEnd
	}

	brk.CreatedAt = now
	brk.UpdatedAt = now

	brk.SetTenantID(tenant.FromContext(ctx))
	if err := s.breakRepo.Create(ctx, brk); err != nil {
		return nil, fmt.Errorf("failed to create break: %w", err)
	}

	s.broadcastTimeTrackingChanged(ctx)
	return brk, nil
}

// EndBreak ends the active break on the running block, whichever day it was
// opened on (see openBlockDay).
func (s *workSessionService) EndBreak(ctx context.Context, staffID int64) (*WorkSession, error) {
	day, err := s.openBlockDay(ctx, staffID)
	if err != nil {
		return nil, err
	}
	return s.EndBreakOn(ctx, staffID, day)
}

// EndBreakOn ends the active break on the open session of an explicit calendar day.
func (s *workSessionService) EndBreakOn(ctx context.Context, staffID int64, day calendar.Date) (*WorkSession, error) {
	if err := s.lockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	// Get the open session of the requested day
	session, err := s.repo.GetOpenByStaffAndDate(ctx, staffID, day)
	if err != nil {
		if isRecordNotFound(err) {
			return nil, workforce.ErrNoActiveSession
		}
		return nil, fmt.Errorf(errGetCurrentSession, err)
	}
	if session == nil {
		return nil, workforce.ErrNoActiveSession
	}

	// Find active break
	activeBreak, err := s.breakRepo.GetActiveBySessionID(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active break: %w", err)
	}
	if activeBreak == nil {
		return nil, workforce.ErrNoActiveBreak
	}

	// End the break
	now := time.Now()
	duration := int(math.Round(now.Sub(activeBreak.StartedAt).Minutes()))
	if err := s.breakRepo.EndBreak(ctx, activeBreak.ID, now, duration); err != nil {
		return nil, fmt.Errorf("failed to end break: %w", err)
	}

	// Recalculate break_minutes cache on session
	if err := s.recalcBreakMinutes(ctx, session.ID); err != nil {
		return nil, fmt.Errorf("failed to update break minutes: %w", err)
	}

	// Re-fetch updated session
	updatedSession, err := s.repo.FindByID(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated session: %w", err)
	}

	s.broadcastTimeTrackingChanged(ctx)
	return updatedSession, nil
}

// GetSessionBreaks returns all breaks for a given session
func (s *workSessionService) GetSessionBreaks(ctx context.Context, staffID, sessionID int64) ([]*WorkSessionBreak, error) {
	// Verify ownership: session must belong to requesting staff
	session, err := s.repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, s.handleSessionNotFoundError(err)
	}
	if session.StaffID != staffID {
		return nil, workforce.ErrSessionNotOwned
	}

	breaks, err := s.breakRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session breaks: %w", err)
	}
	return breaks, nil
}

// recalcBreakMinutes sums the ENDED break durations for a session and updates
// the cache. A still-running break contributes nothing: its elapsed time is
// live data, and readers that need it add it themselves against their own
// clock (netMinutes + runningBreakMinutes in the month service, the live
// session card). Folding the running duration in here would double-count it —
// once from the cache, once from the reader — for every recalc that happens
// while a break is open (editing an ended break on a session whose second
// break is still running), and would leave a stale snapshot in the cache the
// moment the recalc returned.
func (s *workSessionService) recalcBreakMinutes(ctx context.Context, sessionID int64) error {
	breaks, err := s.breakRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get breaks for recalc: %w", err)
	}

	totalMinutes := 0
	for _, brk := range breaks {
		if brk.EndedAt != nil {
			totalMinutes += brk.DurationMinutes
		}
	}

	return s.repo.UpdateBreakMinutes(ctx, sessionID, totalMinutes)
}

// AutoEndExpiredBreaks ends all breaks whose planned_end_time has passed
// Returns the number of breaks that were ended
func (s *workSessionService) AutoEndExpiredBreaks(ctx context.Context) (int, error) {
	now := time.Now()

	// Get all expired breaks
	expiredBreaks, err := s.breakRepo.GetExpiredBreaks(ctx, now)
	if err != nil {
		return 0, fmt.Errorf("failed to get expired breaks: %w", err)
	}

	if len(expiredBreaks) == 0 {
		return 0, nil
	}
	sessionIDs := make([]int64, 0, len(expiredBreaks))
	for _, brk := range expiredBreaks {
		sessionIDs = append(sessionIDs, brk.SessionID)
	}
	sessions, err := s.repo.ListWithFilter(ctx, workforce.WorkSessionFilter{IDs: sessionIDs})
	if err != nil {
		return 0, fmt.Errorf("failed to resolve work sessions for balance lock: %w", err)
	}
	sessionsByID := make(map[int64]*WorkSession, len(sessions))
	for _, session := range sessions {
		sessionsByID[session.ID] = session
	}
	staffIDs := make([]int64, 0, len(expiredBreaks))
	for _, brk := range expiredBreaks {
		session, ok := sessionsByID[brk.SessionID]
		if !ok {
			return 0, fmt.Errorf("failed to resolve work session %d for balance lock", brk.SessionID)
		}
		staffIDs = append(staffIDs, session.StaffID)
	}
	if err := s.lockStaffBalanceWritesOrdered(ctx, staffIDs); err != nil {
		return 0, err
	}

	count := 0
	for _, brk := range expiredBreaks {
		// Use planned_end_time as the end time for accurate duration
		endTime := *brk.PlannedEndTime
		duration := int(math.Round(endTime.Sub(brk.StartedAt).Minutes()))

		if err := s.breakRepo.EndBreak(ctx, brk.ID, endTime, duration); err != nil {
			s.getLogger().WarnContext(ctx, "failed to auto-end break",
				slog.Int64("break_id", brk.ID),
				slog.String("error", err.Error()))
			continue
		}

		// Recalculate break minutes for the session
		if err := s.recalcBreakMinutes(ctx, brk.SessionID); err != nil {
			s.getLogger().WarnContext(ctx, "failed to recalc break minutes",
				slog.Int64("session_id", brk.SessionID),
				slog.String("error", err.Error()))
		}

		count++
	}

	if count > 0 {
		s.broadcastTimeTrackingChanged(ctx)
	}
	return count, nil
}
