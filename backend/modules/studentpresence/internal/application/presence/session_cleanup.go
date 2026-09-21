package presence

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

// CleanupAbandonedSessions cleans up sessions that have been abandoned for longer than the specified duration.
// A session is considered abandoned if:
// 1. No activity (RFID scans or device pings) for longer than the threshold, AND
// 2. The device is offline (not pinging)
// This ensures sessions stay alive if either there's activity OR the device is still online.
func (s *service) CleanupAbandonedSessions(ctx context.Context, threshold time.Duration) (int, error) {
	// Find sessions with no activity since the threshold
	cutoffTime := time.Now().Add(-threshold)
	sessions, err := s.GroupRepo.FindActiveSessionsOlderThan(ctx, cutoffTime)
	if err != nil {
		return 0, &ActiveError{Op: "CleanupAbandonedSessions", Err: err}
	}

	cleanedCount := 0
	for _, session := range sessions {
		// Session is abandoned only if BOTH conditions are true:
		// 1. No recent activity (already filtered by query)
		// 2. Device is offline (not pinging)
		deviceOnline := s.isDeviceOnline(ctx, session.Device, time.Now())
		if deviceOnline {
			// Device is still pinging - session stays alive
			continue
		}

		// Both conditions met: no activity AND device offline - clean up
		// Use ProcessSessionTimeoutByID with the session ID directly to prevent TOCTOU race condition
		// This ensures we end the exact session we identified as abandoned, not whatever
		// session happens to be current for the device at cleanup time
		//
		_, err := s.ProcessSessionTimeoutByID(ctx, session.ID)
		if err != nil {
			// Log error but continue with other sessions
			// Note: ErrActiveGroupAlreadyEnded is expected if session was ended between
			// identification and cleanup - this is the race condition we're protecting against
			continue
		}
		cleanedCount++
	}

	return cleanedCount, nil
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

	if len(activeIDs) == 0 {
		return result, nil
	}
	slices.Sort(activeIDs)
	if err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		lockedIDs, err := s.lockActiveSessionIDs(txCtx, activeIDs)
		if err != nil {
			return err
		}
		return s.endDailySessionsLocked(txCtx, lockedIDs, result)
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
	if len(activeIDs) == 0 {
		return nil
	}

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
	today := timezone.TodayDate()

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

func (s *service) closeStaleSupervisor(ctx context.Context, record *ports.GroupSupervisor, today timezone.Date) (bool, error) {
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
