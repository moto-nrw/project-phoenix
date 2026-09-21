package presence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Activity Session Management with Conflict Detection

// broadcastActivityStartEvent broadcasts SSE event for activity start
func (s *service) broadcastActivityStartEvent(ctx context.Context, group *active.Group, supervisorIDs []int64) {
	if s.Broadcaster == nil || group == nil {
		return
	}

	activeGroupID := fmt.Sprintf("%d", group.ID)
	roomIDStr := fmt.Sprintf("%d", group.RoomID)

	supervisorIDStrs := make([]string, len(supervisorIDs))
	for i, id := range supervisorIDs {
		supervisorIDStrs[i] = fmt.Sprintf("%d", id)
	}

	realtimeevents.PublishActivityStart(ctx, s.Broadcaster, s.getLogger(), realtimeevents.ActivitySession{
		ActiveGroupID: activeGroupID,
		ActivityName:  s.getActivityName(ctx, group.GroupID),
		RoomID:        roomIDStr,
		RoomName:      s.getRoomName(ctx, group.RoomID),
		SupervisorIDs: supervisorIDStrs,
	})

	// Notify every client of the tenant (including zero-topic) so dashboards
	// refresh. No group scope: a session start affects room occupancy across
	// groups, so clients fall back to a broad refresh (#2057).
	s.broadcastSupervisionRefresh(ctx, activeGroupID, activeSupervisionReasonActivityStarted, nil)
}

// validateSupervisorIDs validates that all supervisor IDs exist as staff members
func (s *service) validateSupervisorIDs(ctx context.Context, supervisorIDs []int64) error {
	if len(supervisorIDs) == 0 {
		return &ActiveError{Op: "ValidateSupervisors", Err: fmt.Errorf("at least one supervisor is required")}
	}

	// Deduplicate supervisor IDs
	uniqueIDs := make(map[int64]bool)
	for _, id := range supervisorIDs {
		uniqueIDs[id] = true
	}

	// Batch-validate all unique supervisor IDs in a single query
	idSlice := slices.Collect(maps.Keys(uniqueIDs))

	staffMap, err := s.StaffRepo.ExistingStaffIDs(ctx, idSlice)
	if err != nil {
		return &ActiveError{Op: "ValidateSupervisors", Err: ErrStaffNotFound}
	}

	// Check that every requested ID was found
	for _, id := range idSlice {
		if _, found := staffMap[id]; !found {
			return &ActiveError{Op: "ValidateSupervisors", Err: ErrStaffNotFound}
		}
	}

	return nil
}

// StartActivitySessionWithSupervisors starts an activity session with multiple supervisors
func (s *service) StartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*active.Group, error) {
	if err := s.validateSupervisorIDs(ctx, supervisorIDs); err != nil {
		return nil, err
	}

	var newGroup *active.Group
	err := s.executeSessionStart(ctx, activityID, deviceID, roomID, "StartActivitySessionWithSupervisors", supervisorIDs, func(ctx context.Context, finalRoomID int64) (*active.Group, error) {
		group, err := s.createSessionWithMultipleSupervisors(ctx, activityID, deviceID, supervisorIDs, finalRoomID)
		newGroup = group
		return group, err
	})

	if err != nil {
		return nil, err
	}

	s.broadcastActivityStartEvent(ctx, newGroup, supervisorIDs)
	return newGroup, nil
}

// executeSessionStart handles common session start logic: conflict checking, device validation, and room determination
// Uses PostgreSQL advisory locks to prevent race conditions when multiple requests try to start the same activity concurrently.
// Wraps all operations in a transaction (via TxHandler.RunInTx) so the advisory lock is always available.
// If a transaction already exists in context (e.g. from handler-level WithTenantTx), it is reused.
func (s *service) executeSessionStart(ctx context.Context, activityID, deviceID int64, roomID *int64, operation string, supervisorIDs []int64, createSession func(context.Context, int64) (*active.Group, error)) error {
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if err := s.acquireActivitySessionLock(txCtx, activityID, operation); err != nil {
			return err
		}
		if err := s.lockSupervisorsForAssignment(txCtx, supervisorIDs); err != nil {
			return err
		}

		// Check for conflicts inside the transaction with the lock held.
		// A kiosk start may join an independent room stay of this activity
		// (phone-created Schulhof / offener Raum). A device-less planner or
		// app session of the same activity is a real conflict: attaching the
		// kiosk would let ending the kiosk close that offering.
		if err := s.ensureDeviceHasNoSession(txCtx, deviceID, operation); err != nil {
			return err
		}

		finalRoomID, err := s.determineRoomIDWithStrategy(txCtx, activityID, roomID, RoomConflictIgnore, true)
		if err != nil {
			return err
		}
		if err := s.SchoolPresence.LockRoomSessionWrites(txCtx, finalRoomID); err != nil {
			return &ActiveError{Op: operation, Err: ErrDatabaseOperation}
		}
		if err := s.checkSessionStartConflicts(txCtx, activityID, deviceID, finalRoomID, operation); err != nil {
			return err
		}

		_, err = createSession(txCtx, finalRoomID)
		return err
	})
	return markRollbackOnRoomCapacity(ctx, err)
}

// ensureDeviceHasNoSession rejects a start on a device that already runs a
// session.
func (s *service) ensureDeviceHasNoSession(ctx context.Context, deviceID int64, operation string) error {
	existingDeviceSession, err := s.GroupRepo.FindActiveByDeviceID(ctx, deviceID)
	if err != nil {
		return &ActiveError{Op: operation, Err: err}
	}
	if existingDeviceSession != nil {
		return &ActiveError{Op: operation, Err: ErrSessionConflict}
	}
	return nil
}

// checkSessionStartConflicts rejects a start that collides with the room's
// sessions: a joinable room stay bound to another device, the activity running
// elsewhere, or another session occupying the room.
func (s *service) checkSessionStartConflicts(ctx context.Context, activityID, deviceID, finalRoomID int64, operation string) error {
	activityIsSystem, err := s.systemActivity(ctx, activityID)
	if err != nil {
		return &ActiveError{Op: operation, Err: err}
	}

	groups, err := s.GroupRepo.FindActiveByRoomID(ctx, finalRoomID)
	if err != nil {
		return &ActiveError{Op: operation, Err: err}
	}
	joinable := joinableOpenSession(groups, activityID, activityIsSystem)
	if joinable != nil && joinable.DeviceID != nil && *joinable.DeviceID != deviceID {
		return &ActiveError{Op: operation, Err: ErrSessionConflict}
	}

	existingActivitySessions, err := s.GroupRepo.FindActiveByGroupID(ctx, activityID)
	if err != nil {
		return &ActiveError{Op: operation, Err: err}
	}
	if activityRunningElsewhere(existingActivitySessions, finalRoomID, deviceID, activityIsSystem) {
		return &ActiveError{Op: operation, Err: ErrSessionConflict}
	}

	excludeID := int64(0)
	if joinable != nil {
		excludeID = joinable.ID
	}
	hasConflict, _, err := s.GroupRepo.CheckRoomConflict(ctx, finalRoomID, excludeID)
	if err != nil {
		return &ActiveError{Op: operation, Err: err}
	}
	if hasConflict {
		return &ActiveError{Op: operation, Err: ErrRoomConflict}
	}
	return nil
}

func (s *service) acquireActivitySessionLock(ctx context.Context, activityID int64, operation string) error {
	if s.SessionStartLock == nil {
		return &ActiveError{Op: operation, Err: errors.New("session start lock repository is not configured")}
	}
	if err := s.SessionStartLock.LockSessionStart(ctx, activityID); err != nil {
		return &ActiveError{Op: operation, Err: fmt.Errorf("failed to acquire activity lock: %w", err)}
	}
	return nil
}

// createSessionWithMultipleSupervisors creates a new session with multiple supervisors and transfers visits
func (s *service) createSessionWithMultipleSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID int64) (*active.Group, error) {
	newGroup, transferredCount, err := s.createSessionBase(ctx, activityID, deviceID, roomID)
	if err != nil {
		return nil, err
	}

	s.assignMultipleSupervisorsNonCritical(ctx, newGroup.ID, supervisorIDs, newGroup.StartTime)

	if transferredCount > 0 {
		s.getLogger().InfoContext(ctx, "visits transferred to new session",
			slog.Int("count", transferredCount),
			slog.Int64("session_id", newGroup.ID),
		)
	}

	return newGroup, nil
}

// assignMultipleSupervisorsNonCritical assigns multiple supervisors but doesn't fail if assignment fails.
// Each supervisor is inserted independently so one bad row doesn't prevent the other valid assignments.
//
// Mirrors the deleted single-supervisor variant's NFC auto-check-in:
// the kiosk-driven IoT activity start dispatches through this path, so each
// supervisor must get a work_session stamped with source='nfc' for the audit
// trail to distinguish kiosk scans from app check-ins. Without the loop here,
// /api/iot/* started sessions silently miss the NFC stamp (Issue #1368).
func (s *service) assignMultipleSupervisorsNonCritical(ctx context.Context, groupID int64, supervisorIDs []int64, startDate time.Time) {
	uniqueSupervisors := make(map[int64]bool)
	for _, id := range supervisorIDs {
		uniqueSupervisors[id] = true
	}

	s.getLogger().DebugContext(ctx, "assigning multiple supervisors",
		slog.Any("supervisor_ids", supervisorIDs),
		slog.Int("unique_count", len(uniqueSupervisors)),
	)

	for staffID := range uniqueSupervisors {
		s.assignSupervisorNonCritical(ctx, groupID, staffID, startDate)
	}
}

func (s *service) assignSupervisorNonCritical(ctx context.Context, groupID, staffID int64, startDate time.Time) {
	supervisor := &active.GroupSupervisor{
		StaffID: staffID, GroupID: groupID, Role: "supervisor",
		StartDate: timezone.DateFromTime(startDate),
	}
	supervisor.SetTenantID(tenant.FromContext(ctx))
	s.runBestEffortDB(ctx, "assign_supervisor", func() error {
		return s.SupervisorRepo.Create(ctx, supervisor)
	}, func(err error) {
		s.getLogger().WarnContext(ctx, "supervisor assignment failed",
			slog.Int64("staff_id", staffID),
			slog.Int64("group_id", groupID),
			slog.String("error", err.Error()),
		)
	})
	if s.WorkSessionService != nil {
		s.ensureNFCAutoCheckInNonCritical(ctx, groupID, staffID)
	}
}

func (s *service) ensureNFCAutoCheckInNonCritical(ctx context.Context, groupID, staffID int64) {
	s.runBestEffortDB(ctx, "nfc_auto_checkin", func() error {
		return s.ensureNFCAutoCheckIn(ctx, groupID, staffID)
	}, func(err error) {
		s.getLogger().WarnContext(ctx, "NFC auto-check-in failed",
			slog.Int64("staff_id", staffID),
			slog.Int64("group_id", groupID),
			slog.String("error", err.Error()),
		)
	})
}

func (s *service) ensureNFCAutoCheckIn(ctx context.Context, groupID, staffID int64) error {
	session, err := s.WorkSessionService.EnsureCheckedIn(ctx, staffID, stampSourceNFC)
	var plannedStart PlannedStartNotReachedError
	if errors.As(err, &plannedStart) {
		plannedStartTime, currentTime := plannedStart.PlannedStartNotReached()
		s.getLogger().InfoContext(ctx, "NFC auto-check-in skipped: planned start not reached",
			slog.Int64("staff_id", staffID),
			slog.Int64("group_id", groupID),
			slog.String("planned_start_time", plannedStartTime),
			slog.String("current_time", currentTime),
		)
		return nil
	}
	if err != nil {
		return err
	}
	if session == nil {
		s.getLogger().InfoContext(ctx, "NFC auto-check-in skipped: staff already checked out today",
			slog.Int64("staff_id", staffID),
			slog.Int64("group_id", groupID),
		)
	}
	return nil
}

// createSessionBase creates a new active group session and transfers visits from recent sessions.
// A kiosk start reuses an independent room stay of this activity so the phone
// move and the Schulhof kiosk share one session. Device-less planner/app
// sessions of the same activity are not joinable.
func (s *service) createSessionBase(ctx context.Context, activityID, deviceID, roomID int64) (*active.Group, int, error) {
	groups, err := s.GroupRepo.FindActiveByRoomID(ctx, roomID)
	if err != nil {
		return nil, 0, err
	}
	activityIsSystem, err := s.systemActivity(ctx, activityID)
	if err != nil {
		return nil, 0, err
	}
	if existing := joinableOpenSession(groups, activityID, activityIsSystem); existing != nil {
		if existing.DeviceID != nil && *existing.DeviceID != deviceID {
			return nil, 0, ErrSessionConflict
		}
		if existing.DeviceID == nil && deviceID > 0 {
			existing.DeviceID = &deviceID
			if err := s.GroupRepo.Update(ctx, existing); err != nil {
				return nil, 0, err
			}
		}
		return s.finishSessionStart(ctx, existing, deviceID, roomID)
	}

	now := time.Now()
	newGroup := &active.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		GroupID:        &activityID,
		DeviceID:       &deviceID,
		RoomID:         roomID,
	}

	newGroup.SetTenantID(tenant.FromContext(ctx))
	if err := s.GroupRepo.Create(ctx, newGroup); err != nil {
		return nil, 0, err
	}
	return s.finishSessionStart(ctx, newGroup, deviceID, roomID)
}

func (s *service) finishSessionStart(ctx context.Context, group *active.Group, deviceID, roomID int64) (*active.Group, int, error) {
	if deviceID > 0 {
		s.updateDeviceLocation(ctx, deviceID, roomID)
	}

	transferredCount, err := s.SchoolPresence.TransferRecentDeviceVisits(ctx, group.ID, deviceID)
	if err != nil {
		return nil, 0, err
	}
	if transferredCount > 0 {
		if err := s.ensureRoomCapacity(ctx, roomID, 0); err != nil {
			return nil, 0, err
		}
	}

	return group, int(transferredCount), nil
}

func activityRunningElsewhere(sessions []*active.Group, roomID, deviceID int64, activityIsSystem bool) bool {
	for _, session := range sessions {
		if session == nil {
			continue
		}
		sameDevice := session.DeviceID != nil && *session.DeviceID == deviceID
		if session.RoomID == roomID && (sameDevice || session.IsIndependentRoomSession(activityIsSystem)) {
			continue
		}
		return true
	}
	return false
}

// activityConflictDestination is the room the kiosk preflight compares
// against: the activity's planned room when set, otherwise the single room
// the running copies already occupy. A split across rooms stays a conflict.
func activityConflictDestination(sessions []*active.Group, plannedRoomID int64) int64 {
	if plannedRoomID > 0 {
		return plannedRoomID
	}
	var roomID int64
	for _, session := range sessions {
		if session == nil || session.RoomID <= 0 {
			continue
		}
		if roomID == 0 {
			roomID = session.RoomID
			continue
		}
		if session.RoomID != roomID {
			return 0
		}
	}
	return roomID
}

// updateDeviceLocation updates the device's room_id to track its last-used location.
// This is fire-and-forget: a failure here should not block session creation.
func (s *service) updateDeviceLocation(ctx context.Context, deviceID, roomID int64) {
	s.runBestEffortDB(ctx, "update_device_location", func() error {
		return s.DeviceRepo.UpdateRoomID(ctx, deviceID, roomID)
	}, func(err error) {
		s.getLogger().WarnContext(ctx, "failed to update device location",
			slog.Int64("device_id", deviceID),
			slog.Int64("room_id", roomID),
			slog.String("error", err.Error()),
		)
	})
}

func (s *service) runBestEffortDB(ctx context.Context, label string, fn func() error, logFailure func(error)) {
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		if err := fn(); err != nil {
			logFailure(err)
		}
		return
	}

	var operationErr error
	err := tenant.WithSavepoint(ctx, func(context.Context) error {
		operationErr = fn()
		return operationErr
	})
	if operationErr != nil {
		logFailure(operationErr)
	}
	if err != nil {
		if errors.Is(err, tenant.ErrSavepointControl) {
			s.getLogger().WarnContext(ctx, "failed to control savepoint for best-effort operation",
				slog.String("operation", label),
				slog.String("error", err.Error()),
			)
		}
	}
}
