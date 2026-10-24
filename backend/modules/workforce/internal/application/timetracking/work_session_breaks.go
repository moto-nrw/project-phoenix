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

func (s *workSessionService) StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (*WorkSessionBreak, error) {
	day, err := s.openBlockDay(ctx, staffID)
	if err != nil {
		return nil, err
	}
	return s.StartBreakOn(ctx, staffID, day, plannedDurationMinutes)
}

func (s *workSessionService) StartBreakOn(ctx context.Context, staffID int64, day calendar.Date, plannedDurationMinutes *int) (*WorkSessionBreak, error) {
	if err := s.lockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	// Get the open session of the requested day
	sessionRecords, err := s.repo.LockOpenWorkSessionOn(ctx, staffID, day.String())
	session := workSessionReadRecord(sessionRecords, err)
	err = rowReadError("get current by staff ID", err, workforce.ErrWorkSessionNotFound)
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
	activeBreakRecords, err := s.breakRepo.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: session.ID, Active: boolPointer(true), Limit: 1})
	activeBreak := firstWorkSessionBreak(activeBreakRecords)
	err = rowReadError("get active break by session ID", err, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check active break: %w", err)
	}
	if activeBreak != nil {
		return nil, workforce.ErrBreakAlreadyActive
	}

	return s.startSessionBreak(ctx, session.ID, plannedDurationMinutes)
}

// startSessionBreak validates and records the newly opened pause, then emits
// the same losable refresh hint as the surrounding clock commands.
func (s *workSessionService) startSessionBreak(ctx context.Context, sessionID int64, plannedDurationMinutes *int) (*WorkSessionBreak, error) {
	// Validate plannedDurationMinutes if provided
	if plannedDurationMinutes != nil {
		if *plannedDurationMinutes < 1 || *plannedDurationMinutes > maxPlannedBreakDurationMinutes {
			return nil, &workforce.TimeTrackingError{Kind: workforce.ErrPlannedBreakDurationInvalid, Cause: fmt.Errorf("planned_duration_minutes must be between 1 and %d", maxPlannedBreakDurationMinutes)}
		}
	}

	// Create a new break
	now := s.now()
	brk := &WorkSessionBreak{
		SessionID: sessionID,
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
	if err := brk.Validate(); err != nil {
		return nil, fmt.Errorf("failed to create break: %w", err)
	}
	stored, err := s.breakRepo.CreateWorkSessionBreak(ctx, WorkSessionBreakToCapability(brk))
	err = workSessionWriteError("create", err)
	if err != nil {
		return nil, fmt.Errorf("failed to create break: %w", err)
	}
	applyWorkSessionBreakToRecord(brk, stored)

	s.broadcastTimeTrackingChanged(ctx)
	return brk, nil
}

func (s *workSessionService) EndBreak(ctx context.Context, staffID int64) (*WorkSession, error) {
	day, err := s.openBlockDay(ctx, staffID)
	if err != nil {
		return nil, err
	}
	return s.EndBreakOn(ctx, staffID, day)
}

func (s *workSessionService) EndBreakOn(ctx context.Context, staffID int64, day calendar.Date) (*WorkSession, error) {
	if err := s.lockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	// Get the open session of the requested day
	sessionRecords, err := s.repo.OpenWorkSessionOn(ctx, staffID, day.String())
	session := workSessionReadRecord(sessionRecords, err)
	err = rowReadError("get current by staff ID", err, workforce.ErrWorkSessionNotFound)
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
	activeBreakRecords, err := s.breakRepo.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: session.ID, Active: boolPointer(true), Limit: 1})
	activeBreak := firstWorkSessionBreak(activeBreakRecords)
	err = rowReadError("get active break by session ID", err, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get active break: %w", err)
	}
	if activeBreak == nil {
		return nil, workforce.ErrNoActiveBreak
	}

	// End the break
	now := time.Now()
	duration := int(math.Round(now.Sub(activeBreak.StartedAt).Minutes()))
	affected, err := s.breakRepo.EndWorkSessionBreak(ctx, activeBreak.ID, now, duration)
	err = breakMutationError("end break", affected, err)
	if err != nil {
		return nil, fmt.Errorf("failed to end break: %w", err)
	}

	// Recalculate break_minutes cache on session
	if err := s.recalcBreakMinutes(ctx, session.ID); err != nil {
		return nil, fmt.Errorf("failed to update break minutes: %w", err)
	}

	// Re-fetch updated session
	updatedSessionRecords, err := s.repo.FindWorkSession(ctx, session.ID)
	updatedSession := workSessionReadRecord(updatedSessionRecords, err)
	err = rowReadError("find by id", err, workforce.ErrWorkSessionNotFound)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated session: %w", err)
	}

	s.broadcastTimeTrackingChanged(ctx)
	return updatedSession, nil
}

func (s *workSessionService) GetSessionBreaks(ctx context.Context, staffID, sessionID int64) ([]*WorkSessionBreak, error) {
	// Verify ownership: session must belong to requesting staff
	sessionRecords, err := s.repo.FindWorkSession(ctx, sessionID)
	session := workSessionReadRecord(sessionRecords, err)
	err = rowReadError("find by id", err, workforce.ErrWorkSessionNotFound)
	if err != nil {
		return nil, s.handleSessionNotFoundError(err)
	}
	if session.StaffID != staffID {
		return nil, workforce.ErrSessionNotOwned
	}

	breaksRecords, err := s.breakRepo.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: sessionID})
	breaks := workSessionBreaksToRecords(breaksRecords)
	err = rowReadError("get breaks by session ID", err, nil)
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
	breaksRecords, err := s.breakRepo.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: sessionID})
	breaks := workSessionBreaksToRecords(breaksRecords)
	err = rowReadError("get breaks by session ID", err, nil)
	if err != nil {
		return fmt.Errorf("failed to get breaks for recalc: %w", err)
	}

	totalMinutes := 0
	for _, brk := range breaks {
		if brk.EndedAt != nil {
			totalMinutes += brk.DurationMinutes
		}
	}

	affected, err := s.repo.SetWorkSessionBreakMinutes(ctx, sessionID, totalMinutes)
	if err != nil {
		return workSessionWriteError("update columns WorkSession", err)
	}
	return breakMutationError("update break minutes", affected, nil)
}

func (s *workSessionService) AutoEndExpiredBreaks(ctx context.Context) (int, error) {
	now := time.Now()

	// Get all expired breaks
	expiredBreaksRecords, err := s.breakRepo.ExpiredWorkSessionBreaks(ctx, now)
	expiredBreaks := workSessionBreaksToRecords(expiredBreaksRecords)
	err = rowReadError("get expired breaks", err, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get expired breaks: %w", err)
	}

	if len(expiredBreaks) == 0 {
		return 0, nil
	}
	if err := s.lockExpiredBreakOwners(ctx, expiredBreaks); err != nil {
		return 0, err
	}

	count := 0
	for _, brk := range expiredBreaks {
		// Use planned_end_time as the end time for accurate duration
		endTime := *brk.PlannedEndTime
		duration := int(math.Round(endTime.Sub(brk.StartedAt).Minutes()))

		affected, err := s.breakRepo.EndWorkSessionBreak(ctx, brk.ID, endTime, duration)
		err = breakMutationError("end break", affected, err)
		if err != nil {
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

// lockExpiredBreakOwners resolves the staff member behind every expired
// break's session and takes their balance locks in order.
func (s *workSessionService) lockExpiredBreakOwners(ctx context.Context, expiredBreaks []*WorkSessionBreak) error {
	sessionIDs := make([]int64, 0, len(expiredBreaks))
	for _, brk := range expiredBreaks {
		sessionIDs = append(sessionIDs, brk.SessionID)
	}
	sessionsRecords, err := s.repo.ListWorkSessions(ctx, workforce.WorkSessionFilter{IDs: sessionIDs})
	sessions := workSessionsToRecords(sessionsRecords)
	err = rowReadError("list with options", err, nil)
	if err != nil {
		return fmt.Errorf("failed to resolve work sessions for balance lock: %w", err)
	}
	sessionsByID := make(map[int64]*WorkSession, len(sessions))
	for _, session := range sessions {
		sessionsByID[session.ID] = session
	}
	staffIDs := make([]int64, 0, len(expiredBreaks))
	for _, brk := range expiredBreaks {
		session, ok := sessionsByID[brk.SessionID]
		if !ok {
			return fmt.Errorf("failed to resolve work session %d for balance lock", brk.SessionID)
		}
		staffIDs = append(staffIDs, session.StaffID)
	}
	return s.lockStaffBalanceWritesOrdered(ctx, staffIDs)
}
