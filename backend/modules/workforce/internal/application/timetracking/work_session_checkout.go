package timetracking

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (s *workSessionService) CheckOut(ctx context.Context, staffID int64, reason string) (*WorkSession, error) {
	day, err := s.openBlockDay(ctx, staffID)
	if err != nil {
		return nil, err
	}
	return s.CheckOutOn(ctx, staffID, day, reason)
}

// openBlockDay resolves the calendar day the currently running block is filed
// on, for the web endpoints that stamp without carrying a day themselves. It
// is the service-side twin of the kiosk's clockDay: the running block's own
// day, or today when nobody is clocked in.
//
// Asking the clock unconditionally would strand every block that crossed
// Berlin midnight: it stays open and is still dated yesterday, so a today-only
// lookup reports "no active session found" for somebody who is demonstrably
// clocked in — leaving no way to check out or end the running break at all.
//
// Finding nothing open is not decided here: the day-scoped lookup each caller
// performs next says "no active session found" on its own, and keeping that
// one authority avoids two lookups disagreeing about whether a stamp exists.
func (s *workSessionService) openBlockDay(ctx context.Context, staffID int64) (calendar.Date, error) {
	open, err := s.repo.GetLatestOpenByStaffID(ctx, staffID)
	if err != nil && !isRecordNotFound(err) {
		return calendar.Date(""), fmt.Errorf(errGetCurrentSession, err)
	}
	if open != nil {
		return open.Date, nil
	}
	return calendar.DateFromTime(s.now()), nil
}

func (s *workSessionService) CheckOutOn(ctx context.Context, staffID int64, day calendar.Date, reason string) (*WorkSession, error) {
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

	now := s.now()

	// F9: checking out more than the tolerance after the planned shift end
	// needs a reason ("später gehen"). Referenced against the session's own
	// date so a forgotten session from yesterday is measured against
	// yesterday's plan.
	deviation, err := s.detectPlannedDeviation(ctx, staffID, session.Date, now, deviationActionCheckOut)
	if err != nil {
		return nil, err
	}
	if deviation != nil && strings.TrimSpace(reason) == "" {
		return nil, deviation.requiredError()
	}

	// End any active break before checkout
	if err := s.endActiveBreakIfExists(ctx, session.ID, now); err != nil {
		return nil, err
	}

	// Close the session using repository method
	closed, err := s.repo.CloseSession(ctx, session.ID, now, false)
	if err != nil {
		return nil, fmt.Errorf("failed to close session: %w", err)
	}
	if !closed {
		return nil, workforce.ErrNoActiveSession
	}

	if deviation != nil {
		if err := s.recordDeviationReason(ctx, session, deviation, reason, now); err != nil {
			return nil, err
		}
	}

	// End all active supervisions for this staff member (fire-and-forget)
	s.endActiveSupervisionsOnCheckout(ctx, staffID)

	// Re-fetch the updated session
	updatedSession, err := s.repo.FindByID(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated session: %w", err)
	}

	s.broadcastTimeTrackingChanged(ctx)
	return updatedSession, nil
}

// plannedDeviation describes a stamp that falls outside the tolerance window
// around the planned shift window of the day (F9).
type plannedDeviation struct {
	action  string
	planned time.Time
	actual  time.Time
	minutes int
}

func (d *plannedDeviation) requiredError() *DeviationReasonRequiredError {
	return &DeviationReasonRequiredError{
		Action:           d.action,
		PlannedTime:      d.planned.In(calendar.Berlin).Format("15:04"),
		ActualTime:       d.actual.In(calendar.Berlin).Format("15:04"),
		DeviationMinutes: d.minutes,
	}
}

// detectPlannedDeviation implements the F9 rule: with the tenant setting
// active, a check-in earlier than the earliest planned shift start minus the
// tolerance, or a check-out later than the latest planned shift end plus the
// tolerance, is a deviation that needs a reason. The plan source is
// schedule.staff_shifts (real start AND end times); cancelled shifts do not
// take place, and days without a shift never deviate ("kein Plan, keine
// Abweichung"). Late arrivals and early
// leaves are deliberately not gated — F9 targets unnoticed extra hours
// ("früher kommen oder später gehen"), and missing time is already visible
// in the saldo.
func (s *workSessionService) detectPlannedDeviation(ctx context.Context, staffID int64, day calendar.Date, now time.Time, action string) (*plannedDeviation, error) {
	if s.settings == nil || s.staffShiftRepo == nil {
		return nil, nil
	}
	enabled, err := s.settings.RequireDeviationReason(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve deviation-reason setting: %w", err)
	}
	if !enabled {
		return nil, nil
	}

	plannedStart, plannedEnd, ok, err := s.plannedDayWindow(ctx, staffID, day)
	if err != nil || !ok {
		return nil, err
	}

	toleranceMinutes, err := s.settings.DeviationToleranceMinutes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve deviation-tolerance setting: %w", err)
	}

	var planned time.Time
	var delta time.Duration
	switch action {
	case deviationActionCheckIn:
		planned, delta = plannedStart, plannedStart.Sub(now) // positive when arriving early
	case deviationActionCheckOut:
		planned, delta = plannedEnd, now.Sub(plannedEnd) // positive when leaving late
	default:
		return nil, fmt.Errorf("unknown deviation action %q", action)
	}

	if delta <= time.Duration(toleranceMinutes)*time.Minute {
		return nil, nil
	}

	return &plannedDeviation{
		action:  action,
		planned: planned,
		actual:  now,
		minutes: int(math.Round(delta.Minutes())),
	}, nil
}

// recordDeviationReason persists the F9 reason as an audit edit on the
// session: old value = planned wall clock, new value = actual wall clock,
// notes = the reason. The stamp and its audit row share the handler's tenant
// transaction, so a failed audit write rolls the stamp back.
func (s *workSessionService) recordDeviationReason(ctx context.Context, session *WorkSession, dev *plannedDeviation, reason string, now time.Time) error {
	planned := dev.planned.In(calendar.Berlin).Format("15:04")
	actual := dev.actual.In(calendar.Berlin).Format("15:04")
	trimmed := strings.TrimSpace(reason)
	edit := &WorkSessionEdit{
		SessionID: session.ID,
		StaffID:   session.StaffID,
		EditedBy:  session.StaffID,
		FieldName: "deviation_reason",
		OldValue:  &planned,
		NewValue:  &actual,
		Notes:     &trimmed,
		CreatedAt: now,
	}
	edit.SetTenantID(tenant.FromContext(ctx))
	if err := s.auditRepo.CreateBatch(ctx, []*WorkSessionEdit{edit}); err != nil {
		return fmt.Errorf("failed to record deviation reason: %w", err)
	}
	return nil
}

// endActiveBreakIfExists ends any active break for a session at endAt and
// recalculates break minutes. endAt must not be after the session's checkout
// time, or break rows end up ending after check_out_time.
func (s *workSessionService) endActiveBreakIfExists(ctx context.Context, sessionID int64, endAt time.Time) error {
	activeBreak, err := s.breakRepo.GetActiveBySessionID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to check active break: %w", err)
	}
	if activeBreak == nil {
		return nil
	}

	// A break started after endAt (e.g. during the auto-checkout grace
	// window) is capped to zero length instead of a negative duration.
	if endAt.Before(activeBreak.StartedAt) {
		endAt = activeBreak.StartedAt
	}
	duration := int(math.Round(endAt.Sub(activeBreak.StartedAt).Minutes()))
	if err := s.breakRepo.EndBreak(ctx, activeBreak.ID, endAt, duration); err != nil {
		return fmt.Errorf("failed to end active break: %w", err)
	}

	if err := s.recalcBreakMinutes(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to update break minutes: %w", err)
	}
	return nil
}

// endActiveSupervisionsOnCheckout ends all active supervisions for a staff member (fire-and-forget).
func (s *workSessionService) endActiveSupervisionsOnCheckout(ctx context.Context, staffID int64) {
	if s.supervisorRepo == nil {
		return
	}
	ended, err := s.endActiveSupervisionsWithGroupLocks(ctx, staffID)
	if err != nil {
		s.getLogger().WarnContext(ctx, "failed to end active supervisions on checkout",
			slog.Int64("staff_id", staffID),
			slog.String("error", err.Error()))
		return
	}
	if ended > 0 {
		s.getLogger().InfoContext(ctx, "ended active supervisions on checkout",
			slog.Int("ended_count", ended),
			slog.Int64("staff_id", staffID))
	}
}

func (s *workSessionService) endActiveSupervisionsWithGroupLocks(ctx context.Context, staffID int64) (int, error) {
	if s.groupRepo == nil {
		return s.supervisorRepo.EndAllActiveByStaffID(ctx, staffID)
	}
	ended := 0
	err := s.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		supervisions, err := s.supervisorRepo.FindActiveByStaffID(txCtx, staffID)
		if err != nil {
			return err
		}
		groupIDs := make([]int64, 0, len(supervisions))
		for _, supervision := range supervisions {
			groupIDs = append(groupIDs, supervision.GroupID)
		}
		if err := s.lockWorkSessionGroupRows(txCtx, groupIDs); err != nil {
			return err
		}
		ended, err = s.supervisorRepo.EndAllActiveByStaffID(txCtx, staffID)
		return err
	})
	return ended, err
}

func (s *workSessionService) lockWorkSessionGroupRows(ctx context.Context, groupIDs []int64) error {
	unique := make(map[int64]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		unique[id] = struct{}{}
	}
	ordered := slices.Collect(maps.Keys(unique))
	slices.Sort(ordered)
	for _, id := range ordered {
		if _, err := s.groupRepo.FindByIDForUpdate(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *workSessionService) runInWorkSessionTx(ctx context.Context, fn func(context.Context) error) error {
	if s.db == nil {
		return fn(ctx)
	}
	return tenant.NewTransactionRunner().RunInTx(ctx, fn)
}

func runTimeTrackingUnit(ctx context.Context, fn func(context.Context) error) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, fn)
}
