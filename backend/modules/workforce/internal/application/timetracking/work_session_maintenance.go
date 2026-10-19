package timetracking

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (s *workSessionService) GetTodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	presenceMap, err := s.repo.GetTodayPresenceMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get today's presence map: %w", err)
	}

	return presenceMap, nil
}

func (s *workSessionService) CleanupOpenSessions(ctx context.Context) (int, error) {
	// Today's Berlin calendar day for the PostgreSQL DATE column
	today := calendar.DateFromTime(s.now())

	// Keep sessions opened yesterday available through today. A night block
	// may cross midnight and must not be closed by the first cleanup run.
	openSessions, err := s.repo.GetOpenSessions(ctx, today.AddDays(-1))
	if err != nil {
		return 0, fmt.Errorf("failed to get open sessions: %w", err)
	}
	staffIDs := make([]int64, 0, len(openSessions))
	for _, session := range openSessions {
		staffIDs = append(staffIDs, session.StaffID)
	}
	if err := s.lockStaffBalanceWritesOrdered(ctx, staffIDs); err != nil {
		return 0, err
	}

	count := 0
	for _, session := range openSessions {
		// Set check-out time to 23:59:59 of the session date in Berlin timezone.
		endOfDay := session.Date.EndOfDay()

		// End any active break before closing the session
		if err := s.endActiveBreakIfExists(ctx, session.ID, endOfDay); err != nil {
			s.getLogger().WarnContext(ctx, "failed to end active break during cleanup",
				slog.Int64("session_id", session.ID),
				slog.String("error", err.Error()))
			// Continue cleanup even if break ending fails
		}

		closed, err := s.repo.CloseSession(ctx, session.ID, endOfDay, true)
		if err != nil {
			return count, fmt.Errorf("failed to close session %d: %w", session.ID, err)
		}
		if closed {
			count++
		}
	}

	if count > 0 {
		s.broadcastTimeTrackingChanged(ctx)
	}
	return count, nil
}

func (s *workSessionService) AutoCheckoutDueSessions(ctx context.Context, grace time.Duration) (int, error) {
	if s.staffShiftRepo == nil {
		return 0, nil
	}

	now := s.now()
	// GetOpenSessions filters date < beforeDate; passing tomorrow includes
	// today's open sessions, which is where forgotten checkouts live.
	tomorrow := calendar.DateFromTime(now).AddDays(1)
	openSessions, err := s.repo.GetOpenSessions(ctx, tomorrow)
	if err != nil {
		return 0, fmt.Errorf("failed to get open sessions: %w", err)
	}
	if len(openSessions) == 0 {
		return 0, nil
	}
	staffIDs := make([]int64, 0, len(openSessions))
	for _, session := range openSessions {
		staffIDs = append(staffIDs, session.StaffID)
	}
	if err := s.lockStaffBalanceWritesOrdered(ctx, staffIDs); err != nil {
		return 0, err
	}

	// Batch the shift lookups per session date; latest shift end per staff wins.
	latestEndByDate := make(map[calendar.Date]map[int64]*TimeTrackingShift)
	byDate := make(map[calendar.Date][]int64)
	for _, session := range openSessions {
		byDate[session.Date] = append(byDate[session.Date], session.StaffID)
	}
	for date, staffIDs := range byDate {
		shifts, err := s.staffShiftRepo.FindByStaffIDsAndDate(ctx, staffIDs, calendar.Date(date))
		if err != nil {
			return 0, fmt.Errorf("failed to load shifts for %s: %w", date.String(), err)
		}
		latest := make(map[int64]*TimeTrackingShift, len(shifts))
		for _, shift := range shifts {
			// A cancelled shift does not take place (#1841): never close a
			// session against the planned end of a shift the person is absent
			// for. The nightly cleanup handles a still-open session instead.
			if shift.Cancelled {
				continue
			}
			current, ok := latest[shift.StaffID]
			if !ok || shift.EndInstant().After(current.EndInstant()) {
				latest[shift.StaffID] = shift
			}
		}
		latestEndByDate[date] = latest
	}

	tenantID := tenant.FromContext(ctx)
	count := 0
	for _, listedSession := range openSessions {
		session, err := s.repo.LockOpenByIDForUpdate(ctx, listedSession.ID)
		if err != nil {
			if isRecordNotFound(err) {
				s.getLogger().InfoContext(ctx, "skipping auto-checkout side effects because session was already closed",
					slog.Int64("session_id", listedSession.ID),
					slog.Int64("staff_id", listedSession.StaffID))
				continue
			}
			return count, fmt.Errorf("failed to lock auto-checkout session %d: %w", listedSession.ID, err)
		}

		shift, ok := latestEndByDate[session.Date][session.StaffID]
		if !ok {
			continue // no shift planned that day — nightly cleanup handles it
		}
		closeAt := shift.EndInstant()
		if !now.After(closeAt.Add(grace)) {
			continue // not yet due
		}
		if !closeAt.After(autoCheckoutEffectiveStart(session)) {
			// Checked in or reopened after the planned shift end — never
			// fabricate a checkout before the effective work start.
			continue
		}

		breaks, err := s.breakRepo.GetBySessionID(ctx, session.ID)
		if err != nil {
			s.getLogger().WarnContext(ctx, "failed to inspect breaks during auto-checkout",
				slog.Int64("session_id", session.ID),
				slog.String("error", err.Error()))
			continue
		}

		_, lateBreak := activeBreakAndLateBreakAfter(breaks, closeAt)
		if lateBreak != nil {
			attrs := []any{
				slog.Int64("session_id", session.ID),
				slog.Int64("break_id", lateBreak.ID),
				slog.Time("break_started_at", lateBreak.StartedAt),
				slog.Time("planned_end", closeAt),
			}
			if lateBreak.EndedAt != nil {
				attrs = append(attrs, slog.Time("break_ended_at", *lateBreak.EndedAt))
			}
			s.getLogger().WarnContext(ctx, "skipping auto-checkout because a break crosses planned shift end", attrs...)
			continue
		}

		closed, err := s.repo.CloseSession(ctx, session.ID, closeAt, true)
		if err != nil {
			return count, fmt.Errorf("failed to auto-checkout session %d: %w", session.ID, err)
		}
		if !closed {
			s.getLogger().InfoContext(ctx, "skipping auto-checkout side effects because session was already closed",
				slog.Int64("session_id", session.ID),
				slog.Int64("staff_id", session.StaffID))
			continue
		}

		breaks, err = s.breakRepo.GetBySessionID(ctx, session.ID)
		if err != nil {
			return count, fmt.Errorf("failed to re-inspect breaks after auto-checkout close for session %d: %w", session.ID, err)
		}
		activeBreak, lateBreak := activeBreakAndLateBreakAfter(breaks, closeAt)
		if lateBreak != nil {
			return count, fmt.Errorf("late break detected after auto-checkout close for session %d", session.ID)
		}

		if activeBreak != nil {
			if err := s.endActiveBreak(ctx, session.ID, activeBreak, closeAt); err != nil {
				return count, fmt.Errorf("failed to end active break during auto-checkout for session %d: %w", session.ID, err)
			}
		}

		// End open supervisions, exactly like manual checkout does.
		s.endActiveSupervisionsOnCheckout(ctx, session.StaffID)

		newVal := closeAt.Format(time.RFC3339)
		note := "Automatische Ausstempelung zum geplanten Schichtende"
		edit := &WorkSessionEdit{
			SessionID: session.ID,
			StaffID:   session.StaffID,
			EditedBy:  0,
			FieldName: "check_out_time",
			OldValue:  nil,
			NewValue:  &newVal,
			Notes:     &note,
		}
		edit.SetTenantID(tenantID)
		if err := s.auditRepo.CreateBatch(ctx, []*WorkSessionEdit{edit}); err != nil {
			// The scheduler runs this inside a per-tenant transaction, so
			// returning here rolls the unaudited checkout back with it.
			return count, fmt.Errorf("failed to write auto-checkout audit entry for session %d: %w", session.ID, err)
		}

		s.getLogger().InfoContext(ctx, "session auto-checked-out at planned shift end",
			slog.Int64("session_id", session.ID),
			slog.Int64("staff_id", session.StaffID),
			slog.String("check_out_time", newVal))
		count++
	}

	if count > 0 {
		s.broadcastTimeTrackingChanged(ctx)
	}
	return count, nil
}

func autoCheckoutEffectiveStart(session *WorkSession) time.Time {
	if session == nil {
		return time.Time{}
	}
	if session.ReopenedAt != nil {
		return *session.ReopenedAt
	}
	return session.CheckInTime
}

func activeBreakAndLateBreakAfter(breaks []*WorkSessionBreak, closeAt time.Time) (*WorkSessionBreak, *WorkSessionBreak) {
	var activeBreak *WorkSessionBreak
	for _, brk := range breaks {
		if brk == nil {
			continue
		}
		if brk.EndedAt == nil {
			activeBreak = brk
		}
		if brk.StartedAt.After(closeAt) {
			return activeBreak, brk
		}
		if brk.EndedAt != nil && brk.EndedAt.After(closeAt) {
			return activeBreak, brk
		}
	}
	return activeBreak, nil
}

func (s *workSessionService) endActiveBreak(ctx context.Context, sessionID int64, activeBreak *WorkSessionBreak, endAt time.Time) error {
	duration := int(math.Round(endAt.Sub(activeBreak.StartedAt).Minutes()))
	if err := s.breakRepo.EndBreak(ctx, activeBreak.ID, endAt, duration); err != nil {
		return fmt.Errorf("failed to end active break: %w", err)
	}

	if err := s.recalcBreakMinutes(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to update break minutes: %w", err)
	}
	return nil
}
