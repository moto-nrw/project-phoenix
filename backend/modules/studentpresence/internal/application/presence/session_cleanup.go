package presence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// CleanupAbandonedSessions cleans up sessions that have been abandoned for longer than the specified duration.
// A session is considered abandoned if:
// 1. No activity (RFID scans or device pings) for longer than the threshold, AND
// 2. The device is offline (not pinging)
// This ensures sessions stay alive if either there's activity OR the device is still online.
func (s *service) CleanupAbandonedSessions(ctx context.Context, threshold time.Duration) (int, error) {
	// Find sessions with no activity since the threshold
	cutoffTime := time.Now().Add(-threshold)
	var sessions []*ports.ActiveGroup
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		var readErr error
		sessions, readErr = s.GroupRepo.FindActiveSessionsOlderThan(txCtx, cutoffTime)
		return readErr
	})
	if err != nil {
		return 0, &ActiveError{Op: "CleanupAbandonedSessions", Err: err}
	}

	// Freeze eligibility once. The scheduler/CLI may share one tenant unit
	// across the whole sweep, so acquire every visit before its first session.
	ids := make([]int64, 0, len(sessions))
	for _, session := range sessions {
		if !s.isDeviceOnline(ctx, session.Device, time.Now()) {
			ids = append(ids, session.ID)
		}
	}
	slices.Sort(ids)
	if _, shared := tenant.TransactionFromContext(ctx); shared && len(ids) > 0 {
		if err := s.SchoolPresence.PrepareVisitWrites(ctx, studentpresence.VisitWriteScope{GroupIDs: ids}); err != nil {
			return 0, &ActiveError{Op: "CleanupAbandonedSessions", Err: err}
		}
	}
	cleanedCount := 0
	for _, id := range ids {
		// Without an ambient unit each timeout still commits independently, as
		// before. An ended or resumed session is an expected skip after
		// discovery.
		_, err := s.ProcessSessionTimeoutByID(ctx, id, s.stillAbandoned(cutoffTime))
		switch {
		case err == nil:
			cleanedCount++
		case !errors.Is(err, ErrActiveGroupAlreadyEnded) && !errors.Is(err, errSessionResumed):
			s.getLogger().WarnContext(ctx, "abandoned session cleanup skipped a session",
				slog.Int64("active_group_id", id),
				slog.String("error", err.Error()),
			)
		}
	}

	return cleanedCount, nil
}

// errSessionResumed skips a sweep candidate that became active again between
// the unlocked selection and the session lock.
var errSessionResumed = errors.New("session is no longer abandoned")

// stillAbandoned re-judges a sweep candidate under its session lock (#6502).
// The selection ran without one: a scan or heartbeat may have refreshed the
// session's activity since, or its device may have come back online. The
// check repeats the selection itself, so both use one predicate. Activity
// writes update the locked row and are ordered before or after this read;
// device contact committed before it is seen, later contact orders after the
// sweep. The sweep rarely has more than a few candidates per school.
func (s *service) stillAbandoned(cutoff time.Time) sessionTimeoutCheck {
	return func(ctx context.Context, session *ports.ActiveGroup) error {
		candidates, err := s.GroupRepo.FindActiveSessionsOlderThan(ctx, cutoff)
		if err != nil {
			return err
		}
		for _, candidate := range candidates {
			if candidate.ID == session.ID && !s.isDeviceOnline(ctx, candidate.Device, time.Now()) {
				return nil
			}
		}
		return errSessionResumed
	}
}

// isDeviceOnline reports whether the device was online at the supplied
// observation time. A device is online when its last_seen timestamp is within
// the resolved online window of now. The window comes from the per-tenant
// setting iot.device_online_window_minutes, falling back to
// defaultDeviceOnlineWindow when the resolver is nil, no override exists, or
// the lookup fails. Moved off the iot.Device model per issue #586 (Rule 12).
func (s *service) isDeviceOnline(ctx context.Context, device *ports.SessionDevice, now time.Time) bool {
	if device == nil || device.LastSeen == nil {
		return false
	}
	return now.Sub(*device.LastSeen) <= s.deviceOnlineWindow(ctx)
}

// deviceOnlineWindow resolves the per-tenant device-online window, falling back
// to defaultDeviceOnlineWindow.
func (s *service) deviceOnlineWindow(ctx context.Context) time.Duration {
	if s.DeviceRepo != nil {
		if window := s.DeviceRepo.OnlineWindow(ctx); window > 0 {
			return window
		}
	}
	return defaultDeviceOnlineWindow
}

// EndDailySessions ends all active sessions at the end of the day using bulk UPDATEs
func (s *service) EndDailySessions(ctx context.Context) (*DailySessionCleanupResult, error) {
	// Binary-mode tenants don't open activity sessions or visits (L3.1 gates
	// CreateVisit/EndVisit to no-ops), so this job has nothing to close for
	// them. Returning early saves a handful of per-tenant queries on every
	// scheduler tick and keeps the result shape unchanged for callers.
	mode, err := s.GetPresenceMode(ctx)
	if err != nil {
		result := newDailySessionCleanupResult()
		result.Success = false
		return result, &ActiveError{Op: "EndDailySessions", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if mode == PresenceModeBinary {
		return &DailySessionCleanupResult{
			ExecutedAt: time.Now(),
			Success:    true,
			Errors:     make([]string, 0),
		}, nil
	}

	result := newDailySessionCleanupResult()
	activeIDs, err := s.activeSessionIDs(ctx)
	if err != nil {
		result.Success = false
		return result, &ActiveError{Op: "EndDailySessions", Err: ErrDatabaseOperation}
	}
	// Always clean up orphaned supervisors from previous days, regardless of
	// whether today's bulk steps succeed or are skipped.
	defer s.cleanupOrphanedSupervisors(ctx, result)

	// Run the owner's nightly maintenance even when every session was
	// already closed manually; revision events still need compaction.
	slices.Sort(activeIDs)
	if err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		if err := s.SchoolPresence.PrepareVisitWrites(txCtx, studentpresence.VisitWriteScope{GroupIDs: activeIDs}); err != nil {
			return err
		}
		lockedIDs, err := s.lockActiveSessionIDs(txCtx, activeIDs)
		if err != nil {
			return err
		}
		if err := s.endDailySessionsLocked(txCtx, lockedIDs, result); err != nil {
			return err
		}
		if len(lockedIDs) > 0 {
			// One school-wide refresh for the whole day's close (#5017).
			s.queueSupervisionRefresh(txCtx, lockedIDs[0], activeSupervisionReasonActivityEnded)
		}
		return nil
	}); err != nil {
		result.Success = false
		return result, &ActiveError{Op: "EndDailySessions", Err: err}
	}
	return result, nil
}

func newDailySessionCleanupResult() *DailySessionCleanupResult {
	return &DailySessionCleanupResult{
		ExecutedAt: time.Now(),
		Success:    true,
		Errors:     make([]string, 0),
	}
}

func (s *service) activeSessionIDs(ctx context.Context) ([]int64, error) {
	activeGroups, err := s.GroupRepo.FindActiveGroups(ctx)
	if err != nil {
		return nil, err
	}
	activeIDs := make([]int64, 0, len(activeGroups))
	for _, group := range activeGroups {
		activeIDs = append(activeIDs, group.ID)
	}
	return activeIDs, nil
}

func (s *service) lockActiveSessionIDs(ctx context.Context, ids []int64) ([]int64, error) {
	lockedIDs := make([]int64, 0, len(ids))
	for _, id := range ids {
		group, err := s.GroupRepo.FindByIDForUpdate(ctx, id)
		if err != nil {
			return nil, err
		}
		if group != nil && group.IsActive() {
			lockedIDs = append(lockedIDs, id)
		}
	}
	return lockedIDs, nil
}

func (s *service) endDailySessionsLocked(ctx context.Context, activeIDs []int64, result *DailySessionCleanupResult) error {
	// The Student Presence owner closes visits, sessions, and supervisions of
	// every still-open group in one command (#2697); a failure inside rolls
	// the whole batch back with the surrounding transaction, so no session
	// is ever closed while its visits remain open.
	ended, err := s.SchoolPresence.EndGroupSessions(ctx, activeIDs, s.now())
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to bulk-end sessions: %v", err))
		result.Success = false
		return err
	}
	result.VisitsEnded = int(ended.VisitsClosed)
	result.SessionsEnded = int(ended.SessionsEnded)
	result.SupervisorsEnded = int(ended.SupervisorsEnded)
	result.EndedActiveGroupIDs = append(result.EndedActiveGroupIDs, ended.EndedActiveGroupIDs...)

	return nil
}

// cleanupOrphanedSupervisors closes supervisor records from previous days
// that the per-group loop wouldn't find (e.g., groups already ended but supervisors left open)
func (s *service) cleanupOrphanedSupervisors(ctx context.Context, result *DailySessionCleanupResult) {
	today := calendar.TodayDate()

	// Find orphaned supervisor records from before today with no end_date
	staleRecords, err := s.SupervisorRepo.FindStaleOpen(ctx, today)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to find orphaned supervisors: %v", err)
		result.Errors = append(result.Errors, errMsg)
		result.Success = false
		return
	}

	for _, record := range staleRecords {
		closed, err := s.closeStaleSupervisor(ctx, record, today)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to close orphaned supervisor %d: %v", record.ID, err)
			result.Errors = append(result.Errors, errMsg)
			result.Success = false
		} else if closed {
			result.SupervisorsEnded++
		}
	}
}

func (s *service) closeStaleSupervisor(ctx context.Context, record *ports.GroupSupervisor, today calendar.Date) (bool, error) {
	closed := false
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		if err := s.lockGroupRows(txCtx, record.GroupID); err != nil {
			return err
		}
		current, err := s.SupervisorRepo.FindByID(txCtx, record.ID)
		if err != nil {
			return err
		}
		if current == nil || current.GroupID != record.GroupID {
			return ErrGroupSupervisorNotFound
		}
		if current.EndDate != nil || !current.StartDate.Before(today) {
			return nil
		}
		endDate := current.StartDate
		current.EndDate = &endDate
		current.UpdatedAt = time.Now()
		_, err = s.SupervisorRepo.SetEndDate(txCtx, current)
		closed = err == nil
		return err
	})
	return closed, err
}
