package active

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/uptrace/bun"
)

// Activity Session Management with Conflict Detection

// StartActivitySession starts a new activity session on a device with conflict detection
func (s *service) StartActivitySession(ctx context.Context, activityID, deviceID, staffID int64, roomID *int64) (*active.Group, error) {
	var newGroup *active.Group
	err := s.executeSessionStart(ctx, activityID, deviceID, roomID, "StartActivitySession", func(ctx context.Context, finalRoomID int64) (*active.Group, error) {
		group, err := s.createSessionWithSupervisor(ctx, activityID, deviceID, staffID, finalRoomID)
		newGroup = group
		return group, err
	})

	if err != nil {
		return nil, err
	}

	s.broadcastActivityStartEvent(ctx, newGroup, []int64{staffID})
	return newGroup, nil
}

// determineSessionRoomID determines the room for a session with conflict checking
func (s *service) determineSessionRoomID(ctx context.Context, activityID int64, roomID *int64) (int64, error) {
	return s.determineRoomIDWithStrategy(ctx, activityID, roomID, RoomConflictFail)
}

// createSessionWithSupervisor creates a new session, assigns supervisor, and transfers visits
func (s *service) createSessionWithSupervisor(ctx context.Context, activityID, deviceID, staffID, roomID int64) (*active.Group, error) {
	newGroup, transferredCount, err := s.createSessionBase(ctx, activityID, deviceID, roomID)
	if err != nil {
		return nil, err
	}

	s.assignSupervisorNonCritical(ctx, newGroup.ID, staffID, newGroup.StartTime)

	if transferredCount > 0 {
		s.getLogger().InfoContext(ctx, "visits transferred to new session",
			slog.Int("count", transferredCount),
			slog.Int64("session_id", newGroup.ID),
		)
	}

	return newGroup, nil
}

// assignSupervisorNonCritical assigns a supervisor but doesn't fail if assignment fails.
// Also ensures the staff member is checked in for time tracking (NFC auto-check-in).
func (s *service) assignSupervisorNonCritical(ctx context.Context, groupID, staffID int64, startDate time.Time) {
	supervisor := &active.GroupSupervisor{
		StaffID:   staffID,
		GroupID:   groupID,
		Role:      "Supervisor",
		StartDate: startDate,
	}
	if err := s.supervisorRepo.Create(ctx, supervisor); err != nil {
		s.getLogger().WarnContext(ctx, "supervisor assignment failed",
			slog.Int64("staff_id", staffID),
			slog.Int64("group_id", groupID),
			slog.String("error", err.Error()),
		)
	}

	// NFC auto-check-in: ensure staff member has a work session for today
	if s.workSessionService != nil {
		if _, err := s.workSessionService.EnsureCheckedIn(ctx, staffID); err != nil {
			s.getLogger().WarnContext(ctx, "NFC auto-check-in failed",
				slog.Int64("staff_id", staffID),
				slog.String("error", err.Error()),
			)
		}
	}
}

// broadcastActivityStartEvent broadcasts SSE event for activity start
func (s *service) broadcastActivityStartEvent(ctx context.Context, group *active.Group, supervisorIDs []int64) {
	if s.broadcaster == nil || group == nil {
		return
	}

	activeGroupID := fmt.Sprintf("%d", group.ID)
	roomIDStr := fmt.Sprintf("%d", group.RoomID)

	supervisorIDStrs := make([]string, len(supervisorIDs))
	for i, id := range supervisorIDs {
		supervisorIDStrs[i] = fmt.Sprintf("%d", id)
	}

	activityName := s.getActivityName(ctx, group.GroupID)
	roomName := s.getRoomName(ctx, group.RoomID)

	event := realtime.NewEvent(
		realtime.EventActivityStart,
		activeGroupID,
		realtime.EventData{
			ActivityName:  &activityName,
			RoomID:        &roomIDStr,
			RoomName:      &roomName,
			SupervisorIDs: &supervisorIDStrs,
		},
	)

	s.broadcastWithLogging(activeGroupID, "", event, "activity_start")
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

	// Validate each unique supervisor ID exists
	for id := range uniqueIDs {
		_, err := s.staffRepo.FindByID(ctx, id)
		if err != nil {
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
	err := s.executeSessionStart(ctx, activityID, deviceID, roomID, "StartActivitySessionWithSupervisors", func(ctx context.Context, finalRoomID int64) (*active.Group, error) {
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
// Uses PostgreSQL advisory locks to prevent race conditions when multiple requests try to start the same activity concurrently
func (s *service) executeSessionStart(ctx context.Context, activityID, deviceID int64, roomID *int64, operation string, createSession func(context.Context, int64) (*active.Group, error)) error {
	return s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Acquire advisory lock on activity ID to serialize concurrent session starts
		// This prevents race conditions where two requests both pass conflict check before either creates a session
		// The lock is automatically released when the transaction commits or rolls back
		if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", activityID); err != nil {
			return &ActiveError{Op: operation, Err: fmt.Errorf("failed to acquire activity lock: %w", err)}
		}

		// Check for conflicts inside the transaction with the lock held
		conflictInfo, err := s.CheckActivityConflict(ctx, activityID, deviceID)
		if err != nil {
			return &ActiveError{Op: operation, Err: err}
		}
		if conflictInfo.HasConflict {
			return &ActiveError{Op: operation, Err: ErrSessionConflict}
		}

		finalRoomID, err := s.determineSessionRoomID(ctx, activityID, roomID)
		if err != nil {
			return err
		}

		_, err = createSession(ctx, finalRoomID)
		return err
	})
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

// assignMultipleSupervisorsNonCritical assigns multiple supervisors but doesn't fail if assignment fails
func (s *service) assignMultipleSupervisorsNonCritical(ctx context.Context, groupID int64, supervisorIDs []int64, startDate time.Time) {
	// Deduplicate supervisor IDs
	uniqueSupervisors := make(map[int64]bool)
	for _, id := range supervisorIDs {
		uniqueSupervisors[id] = true
	}

	s.getLogger().DebugContext(ctx, "assigning multiple supervisors",
		slog.Any("supervisor_ids", supervisorIDs),
		slog.Int("unique_count", len(uniqueSupervisors)),
	)

	// Assign each unique supervisor
	for staffID := range uniqueSupervisors {
		supervisor := &active.GroupSupervisor{
			StaffID:   staffID,
			GroupID:   groupID,
			Role:      "supervisor",
			StartDate: startDate,
		}
		if err := s.supervisorRepo.Create(ctx, supervisor); err != nil {
			s.getLogger().WarnContext(ctx, "supervisor assignment failed",
				slog.Int64("staff_id", staffID),
				slog.Int64("group_id", groupID),
				slog.String("error", err.Error()),
			)
		}
	}
}

// ForceStartActivitySession starts an activity session with override capability
func (s *service) ForceStartActivitySession(ctx context.Context, activityID, deviceID, staffID int64, roomID *int64) (*active.Group, error) {
	var newGroup *active.Group
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		if err := s.endExistingDeviceSessionIfPresent(ctx, deviceID); err != nil {
			return err
		}

		finalRoomID := s.determineRoomIDWithoutConflictCheck(ctx, activityID, roomID)

		var err error
		newGroup, err = s.createSessionWithSupervisorForceStart(ctx, activityID, deviceID, staffID, finalRoomID)
		return err
	})

	if err != nil {
		return nil, &ActiveError{Op: "ForceStartActivitySession", Err: err}
	}

	return newGroup, nil
}

// endExistingDeviceSessionIfPresent ends any existing session for the device using simple cleanup
func (s *service) endExistingDeviceSessionIfPresent(ctx context.Context, deviceID int64) error {
	return s.endExistingDeviceSession(ctx, deviceID, false)
}

// determineRoomIDWithoutConflictCheck determines room ID without checking conflicts (for force start)
func (s *service) determineRoomIDWithoutConflictCheck(ctx context.Context, activityID int64, roomID *int64) int64 {
	finalRoomID, _ := s.determineRoomIDWithStrategy(ctx, activityID, roomID, RoomConflictIgnore)
	return finalRoomID
}

// createSessionWithSupervisorForceStart creates a session for force start with special logging
func (s *service) createSessionWithSupervisorForceStart(ctx context.Context, activityID, deviceID, staffID, roomID int64) (*active.Group, error) {
	newGroup, transferredCount, err := s.createSessionBase(ctx, activityID, deviceID, roomID)
	if err != nil {
		return nil, err
	}

	s.assignSupervisorNonCritical(ctx, newGroup.ID, staffID, newGroup.StartTime)

	if transferredCount > 0 {
		s.getLogger().InfoContext(ctx, "visits transferred to new session (force start)",
			slog.Int("count", transferredCount),
			slog.Int64("session_id", newGroup.ID),
		)
	}

	return newGroup, nil
}

// createSessionBase creates a new active group session and transfers visits from recent sessions
func (s *service) createSessionBase(ctx context.Context, activityID, deviceID, roomID int64) (*active.Group, int, error) {
	now := time.Now()
	newGroup := &active.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		GroupID:        activityID,
		DeviceID:       &deviceID,
		RoomID:         roomID,
	}

	if err := s.groupRepo.Create(ctx, newGroup); err != nil {
		return nil, 0, err
	}

	transferredCount, err := s.visitRepo.TransferVisitsFromRecentSessions(ctx, newGroup.ID, deviceID)
	if err != nil {
		return nil, 0, err
	}

	return newGroup, transferredCount, nil
}

// ForceStartActivitySessionWithSupervisors starts an activity session with multiple supervisors and override capability
func (s *service) ForceStartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*active.Group, error) {
	s.getLogger().DebugContext(ctx, "force start with multiple supervisors called",
		slog.Any("supervisor_ids", supervisorIDs),
		slog.Int("supervisor_count", len(supervisorIDs)),
		slog.Int64("activity_id", activityID),
		slog.Int64("device_id", deviceID),
	)

	if err := s.validateSupervisorIDs(ctx, supervisorIDs); err != nil {
		return nil, err
	}

	var newGroup *active.Group
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Use simple cleanup (fullCleanup=false) to only mark the group as ended
		// without ending visits, so TransferVisitsFromRecentSessions can move them
		// to the new session. Using fullCleanup=true would set exit_time on all visits
		// first, causing the transfer to find nothing and losing all checked-in students.
		if err := s.endExistingDeviceSessionIfPresent(ctx, deviceID); err != nil {
			return err
		}

		finalRoomID, err := s.determineRoomIDForForceStart(ctx, activityID, roomID)
		if err != nil {
			return err
		}

		newGroup, err = s.createSessionWithMultipleSupervisors(ctx, activityID, deviceID, supervisorIDs, finalRoomID)
		return err
	})

	if err != nil {
		return nil, &ActiveError{Op: "ForceStartActivitySessionWithSupervisors", Err: err}
	}

	return newGroup, nil
}

// endExistingDeviceSession ends any existing session for the device
func (s *service) endExistingDeviceSession(ctx context.Context, deviceID int64, fullCleanup bool) error {
	existingSession, err := s.groupRepo.FindActiveByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}

	if existingSession == nil {
		return nil
	}

	if fullCleanup {
		return s.EndActivitySession(ctx, existingSession.ID)
	}

	return s.groupRepo.EndSession(ctx, existingSession.ID)
}

// determineRoomIDForForceStart determines room ID for force start with conflict warning but no failure
func (s *service) determineRoomIDForForceStart(ctx context.Context, activityID int64, roomID *int64) (int64, error) {
	return s.determineRoomIDWithStrategy(ctx, activityID, roomID, RoomConflictWarn)
}

// determineRoomIDWithStrategy determines room ID with configurable conflict handling strategy
func (s *service) determineRoomIDWithStrategy(ctx context.Context, activityID int64, roomID *int64, strategy RoomConflictStrategy) (int64, error) {
	// Manual room selection has highest priority
	if roomID != nil && *roomID > 0 {
		return s.validateManualRoomSelection(ctx, *roomID, strategy)
	}

	// Try to get planned room from activity configuration
	if plannedRoomID := s.getPlannedRoomID(ctx, activityID); plannedRoomID > 0 {
		return plannedRoomID, nil
	}

	// Default fallback room
	return 1, nil
}

// validateManualRoomSelection validates manually selected room based on conflict strategy
func (s *service) validateManualRoomSelection(ctx context.Context, roomID int64, strategy RoomConflictStrategy) (int64, error) {
	if strategy == RoomConflictIgnore {
		return roomID, nil
	}

	hasConflict, _, err := s.groupRepo.CheckRoomConflict(ctx, roomID, 0)
	if err != nil {
		return 0, err
	}

	if hasConflict {
		if strategy == RoomConflictFail {
			return 0, ErrRoomConflict
		}
		s.getLogger().WarnContext(ctx, "overriding room conflict",
			slog.Int64("room_id", roomID),
		)
	}

	return roomID, nil
}

// getPlannedRoomID retrieves the planned room ID from activity configuration
func (s *service) getPlannedRoomID(ctx context.Context, activityID int64) int64 {
	activityGroup, err := s.activityGroupRepo.FindByID(ctx, activityID)
	if err == nil && activityGroup != nil && activityGroup.PlannedRoomID != nil && *activityGroup.PlannedRoomID > 0 {
		return *activityGroup.PlannedRoomID
	}
	return 0
}

// UpdateActiveGroupSupervisors replaces all supervisors for an active group
func (s *service) UpdateActiveGroupSupervisors(ctx context.Context, activeGroupID int64, supervisorIDs []int64) (*active.Group, error) {
	if err := s.validateActiveGroupForSupervisorUpdate(ctx, activeGroupID); err != nil {
		return nil, err
	}

	if err := s.validateSupervisorIDs(ctx, supervisorIDs); err != nil {
		return nil, err
	}

	uniqueSupervisors := deduplicateSupervisorIDs(supervisorIDs)

	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		return s.replaceSupervisorsInTransaction(ctx, activeGroupID, uniqueSupervisors)
	})

	if err != nil {
		return nil, &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: err}
	}

	updatedGroup, err := s.groupRepo.FindWithSupervisors(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: err}
	}

	return updatedGroup, nil
}

// validateActiveGroupForSupervisorUpdate validates that the group exists and is active
func (s *service) validateActiveGroupForSupervisorUpdate(ctx context.Context, activeGroupID int64) error {
	activeGroup, err := s.groupRepo.FindByID(ctx, activeGroupID)
	if err != nil {
		return &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: ErrActiveGroupNotFound}
	}

	if !activeGroup.IsActive() {
		return &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: fmt.Errorf("cannot update supervisors for an ended session")}
	}

	return nil
}

// deduplicateSupervisorIDs removes duplicate supervisor IDs
func deduplicateSupervisorIDs(supervisorIDs []int64) map[int64]bool {
	uniqueSupervisors := make(map[int64]bool)
	for _, id := range supervisorIDs {
		uniqueSupervisors[id] = true
	}
	return uniqueSupervisors
}

// replaceSupervisorsInTransaction replaces all supervisors for a group within a transaction
func (s *service) replaceSupervisorsInTransaction(ctx context.Context, activeGroupID int64, uniqueSupervisors map[int64]bool) error {
	currentSupervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, activeGroupID, true)
	if err != nil {
		return err
	}

	if err := s.endAllCurrentSupervisors(ctx, currentSupervisors); err != nil {
		return err
	}

	return s.upsertSupervisors(ctx, activeGroupID, uniqueSupervisors, currentSupervisors)
}

// endAllCurrentSupervisors ends all current supervisors by setting end_date
func (s *service) endAllCurrentSupervisors(ctx context.Context, supervisors []*active.GroupSupervisor) error {
	now := time.Now()
	for _, supervisor := range supervisors {
		supervisor.EndDate = &now
		if err := s.supervisorRepo.Update(ctx, supervisor); err != nil {
			return err
		}
	}
	return nil
}

// upsertSupervisors creates new supervisors or reactivates existing ones
func (s *service) upsertSupervisors(ctx context.Context, activeGroupID int64, uniqueSupervisors map[int64]bool, currentSupervisors []*active.GroupSupervisor) error {
	now := time.Now()

	for supervisorID := range uniqueSupervisors {
		existingSuper := s.findExistingSupervisor(currentSupervisors, supervisorID)

		if existingSuper != nil {
			if err := s.reactivateSupervisor(ctx, existingSuper, now); err != nil {
				return err
			}
		} else {
			if err := s.createNewSupervisor(ctx, activeGroupID, supervisorID, now); err != nil {
				return err
			}
		}
	}

	return nil
}

// findExistingSupervisor finds a supervisor in the list by staff ID and role
func (s *service) findExistingSupervisor(supervisors []*active.GroupSupervisor, staffID int64) *active.GroupSupervisor {
	for _, existing := range supervisors {
		if existing.StaffID == staffID && existing.Role == "supervisor" {
			return existing
		}
	}
	return nil
}

// reactivateSupervisor reactivates an ended supervisor
func (s *service) reactivateSupervisor(ctx context.Context, supervisor *active.GroupSupervisor, now time.Time) error {
	if supervisor.EndDate == nil {
		return nil
	}

	supervisor.EndDate = nil
	supervisor.StartDate = now
	return s.supervisorRepo.Update(ctx, supervisor)
}

// createNewSupervisor creates a new supervisor record
func (s *service) createNewSupervisor(ctx context.Context, activeGroupID, supervisorID int64, now time.Time) error {
	supervisor := &active.GroupSupervisor{
		StaffID:   supervisorID,
		GroupID:   activeGroupID,
		Role:      "supervisor",
		StartDate: now,
	}
	return s.supervisorRepo.Create(ctx, supervisor)
}

// CheckActivityConflict checks for conflicts before starting an activity session
func (s *service) CheckActivityConflict(ctx context.Context, activityID, deviceID int64) (*ActivityConflictInfo, error) {
	// Check if device is already running another session
	existingDeviceSession, err := s.groupRepo.FindActiveByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "CheckActivityConflict", Err: err}
	}

	if existingDeviceSession != nil {
		deviceIDStr := fmt.Sprintf("%d", deviceID)
		return &ActivityConflictInfo{
			HasConflict:       true,
			ConflictingGroup:  existingDeviceSession,
			ConflictMessage:   fmt.Sprintf("Device %d is already running another session", deviceID),
			ConflictingDevice: &deviceIDStr,
			CanOverride:       true, // Administrative override is always possible
		}, nil
	}

	// Check if activity is already active on a different device
	existingActivitySessions, err := s.groupRepo.FindActiveByGroupID(ctx, activityID)
	if err != nil {
		return nil, &ActiveError{Op: "CheckActivityConflict", Err: err}
	}

	if len(existingActivitySessions) > 0 {
		// Activity is already active on another device
		existingSession := existingActivitySessions[0]
		var conflictDeviceStr *string
		if existingSession.DeviceID != nil {
			deviceIDStr := fmt.Sprintf("%d", *existingSession.DeviceID)
			conflictDeviceStr = &deviceIDStr
		}
		return &ActivityConflictInfo{
			HasConflict:       true,
			ConflictingGroup:  existingSession,
			ConflictMessage:   fmt.Sprintf("Activity is already active on device %s", getDeviceIDString(existingSession.DeviceID)),
			ConflictingDevice: conflictDeviceStr,
			CanOverride:       true, // Administrative override is always possible
		}, nil
	}

	// No conflicts
	return &ActivityConflictInfo{
		HasConflict: false,
		CanOverride: true,
	}, nil
}

// getDeviceIDString returns a string representation of device ID or "unknown" if nil
func getDeviceIDString(deviceID *int64) string {
	if deviceID == nil {
		return "unknown"
	}
	return fmt.Sprintf("%d", *deviceID)
}

// EndActivitySession ends an active activity session
func (s *service) EndActivitySession(ctx context.Context, activeGroupID int64) error {
	// Verify the session exists and is active
	group, err := s.groupRepo.FindByID(ctx, activeGroupID)
	if err != nil {
		return &ActiveError{Op: "EndActivitySession", Err: ErrActiveGroupNotFound}
	}

	if !group.IsActive() {
		return &ActiveError{Op: "EndActivitySession", Err: ErrActiveGroupAlreadyEnded}
	}

	// Collect active visits BEFORE transaction for SSE broadcasts
	visitsToNotify, err := s.collectActiveVisitsForSSE(ctx, activeGroupID)
	if err != nil {
		return &ActiveError{Op: "EndActivitySession", Err: ErrDatabaseOperation}
	}

	// Use transaction to ensure atomic cleanup
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*service)

		// End all active visits
		for _, visitData := range visitsToNotify {
			if err := txService.visitRepo.EndVisit(ctx, visitData.VisitID); err != nil {
				return err
			}
		}

		// Fetch and end all active supervisors inside the transaction
		activeSupervisors, err := txService.supervisorRepo.FindByActiveGroupID(ctx, activeGroupID, true)
		if err != nil {
			return err
		}
		for _, sup := range activeSupervisors {
			if err := txService.supervisorRepo.EndSupervision(ctx, sup.ID); err != nil {
				return err
			}
		}

		// End the session
		if err := txService.groupRepo.EndSession(ctx, activeGroupID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return &ActiveError{Op: "EndActivitySession", Err: err}
	}

	// Broadcast SSE events (fire-and-forget, outside transaction)
	if s.broadcaster != nil {
		activeGroupIDStr := fmt.Sprintf("%d", activeGroupID)
		s.broadcastStudentCheckoutEvents(activeGroupIDStr, visitsToNotify)
		s.broadcastActivityEndEvent(ctx, activeGroupID, activeGroupIDStr)
	}

	return nil
}

// GetDeviceCurrentSession gets the current active session for a device
func (s *service) GetDeviceCurrentSession(ctx context.Context, deviceID int64) (*active.Group, error) {
	session, err := s.groupRepo.FindActiveByDeviceIDWithNames(ctx, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "GetDeviceCurrentSession", Err: err}
	}

	if session == nil {
		return nil, &ActiveError{Op: "GetDeviceCurrentSession", Err: ErrNoActiveSession}
	}

	return session, nil
}

// ProcessSessionTimeout handles device-triggered session timeout
func (s *service) ProcessSessionTimeout(ctx context.Context, deviceID int64) (*TimeoutResult, error) {
	// Validate device has active session
	session, err := s.GetDeviceCurrentSession(ctx, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "ProcessSessionTimeout", Err: ErrNoActiveSession}
	}

	// Delegate to ProcessSessionTimeoutByID with the session ID
	return s.ProcessSessionTimeoutByID(ctx, session.ID)
}

// validateSessionForTimeout validates that a session exists and is still active.
// Returns the session if valid, or an error if not found or already ended.
func (s *service) validateSessionForTimeout(ctx context.Context, sessionID int64) (*active.Group, error) {
	session, err := s.groupRepo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, &ActiveError{Op: "ProcessSessionTimeoutByID", Err: ErrActiveGroupNotFound}
	}

	if !session.IsActive() {
		return nil, &ActiveError{Op: "ProcessSessionTimeoutByID", Err: ErrActiveGroupAlreadyEnded}
	}

	return session, nil
}

// checkoutActiveVisits ends all active visits for a session and returns the count of students checked out.
func (s *service) checkoutActiveVisits(ctx context.Context, sessionID int64) (int, error) {
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, sessionID)
	if err != nil {
		return 0, err
	}

	studentsCheckedOut := 0
	for _, visit := range visits {
		if !visit.IsActive() {
			continue
		}
		if err := s.visitRepo.EndVisit(ctx, visit.ID); err != nil {
			return 0, err
		}
		studentsCheckedOut++
	}

	return studentsCheckedOut, nil
}

// collectActiveVisitsForSSE gathers visit and student data needed for SSE broadcasts
func (s *service) collectActiveVisitsForSSE(ctx context.Context, sessionID int64) ([]visitSSEData, error) {
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	var result []visitSSEData
	for _, visit := range visits {
		if !visit.IsActive() {
			continue
		}
		data := visitSSEData{
			VisitID:   visit.ID,
			StudentID: visit.StudentID,
		}
		// Query student name for SSE event
		if student, err := s.studentRepo.FindByID(ctx, visit.StudentID); err == nil && student != nil {
			data.Student = student
			if person, err := s.personRepo.FindByID(ctx, student.PersonID); err == nil && person != nil {
				data.Name = fmt.Sprintf("%s %s", person.FirstName, person.LastName)
			}
		}
		result = append(result, data)
	}

	return result, nil
}

// ProcessSessionTimeoutByID handles session timeout by session ID directly.
// This is the preferred method for cleanup operations to avoid TOCTOU race conditions.
// It verifies the session is still active before ending it.
func (s *service) ProcessSessionTimeoutByID(ctx context.Context, sessionID int64) (*TimeoutResult, error) {
	// Collect visit data BEFORE transaction for SSE broadcasts
	visitsToNotify, err := s.collectActiveVisitsForSSE(ctx, sessionID)
	if err != nil {
		// Non-fatal: continue without SSE data
		visitsToNotify = nil
	}

	var result *TimeoutResult
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*service)

		session, err := txService.validateSessionForTimeout(ctx, sessionID)
		if err != nil {
			return err
		}

		studentsCheckedOut, err := txService.checkoutActiveVisits(ctx, sessionID)
		if err != nil {
			return err
		}

		if err := txService.groupRepo.EndSession(ctx, sessionID); err != nil {
			return err
		}

		result = &TimeoutResult{
			SessionID:          sessionID,
			ActivityID:         session.GroupID,
			StudentsCheckedOut: studentsCheckedOut,
			TimeoutAt:          time.Now(),
		}
		return nil
	})

	if err != nil {
		if activeErr, ok := err.(*ActiveError); ok {
			return nil, activeErr
		}
		return nil, &ActiveError{Op: "ProcessSessionTimeoutByID", Err: err}
	}

	// Broadcast SSE events (fire-and-forget, outside transaction)
	if s.broadcaster != nil && result != nil {
		sessionIDStr := fmt.Sprintf("%d", sessionID)
		s.broadcastStudentCheckoutEvents(sessionIDStr, visitsToNotify)
		s.broadcastActivityEndEvent(ctx, sessionID, sessionIDStr)
	}

	return result, nil
}

// UpdateSessionActivity updates the last activity timestamp for a session
func (s *service) UpdateSessionActivity(ctx context.Context, activeGroupID int64) error {
	// Get the current session to validate it exists and is active
	session, err := s.groupRepo.FindByID(ctx, activeGroupID)
	if err != nil {
		return &ActiveError{Op: "UpdateSessionActivity", Err: err}
	}

	if session == nil {
		return &ActiveError{Op: "UpdateSessionActivity", Err: ErrActiveGroupNotFound}
	}

	if !session.IsActive() {
		return &ActiveError{Op: "UpdateSessionActivity", Err: ErrActiveGroupAlreadyEnded}
	}

	// Update last activity timestamp
	return s.groupRepo.UpdateLastActivity(ctx, activeGroupID, time.Now())
}

// ValidateSessionTimeout validates if a timeout request is valid
func (s *service) ValidateSessionTimeout(ctx context.Context, deviceID int64, timeoutMinutes int) error {
	// Validate device has active session
	session, err := s.GetDeviceCurrentSession(ctx, deviceID)
	if err != nil {
		return &ActiveError{Op: "ValidateSessionTimeout", Err: err}
	}

	// Validate timeout parameters
	if timeoutMinutes <= 0 || timeoutMinutes > 480 { // Max 8 hours
		return &ActiveError{Op: "ValidateSessionTimeout", Err: fmt.Errorf("invalid timeout minutes: %d", timeoutMinutes)}
	}

	// Check if session is actually timed out based on inactivity
	timeoutDuration := time.Duration(timeoutMinutes) * time.Minute
	inactivityDuration := time.Since(session.LastActivity)

	if inactivityDuration < timeoutDuration {
		return &ActiveError{Op: "ValidateSessionTimeout", Err: fmt.Errorf("session not yet timed out: %v remaining", timeoutDuration-inactivityDuration)}
	}

	return nil
}

// GetSessionTimeoutInfo provides comprehensive timeout information for a device session
func (s *service) GetSessionTimeoutInfo(ctx context.Context, deviceID int64) (*SessionTimeoutInfo, error) {
	// Get current session
	session, err := s.GetDeviceCurrentSession(ctx, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "GetSessionTimeoutInfo", Err: err}
	}

	// Count active students in the session
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, session.ID)
	if err != nil {
		return nil, &ActiveError{Op: "GetSessionTimeoutInfo", Err: err}
	}

	activeStudentCount := 0
	for _, visit := range visits {
		if visit.IsActive() {
			activeStudentCount++
		}
	}

	info := &SessionTimeoutInfo{
		SessionID:          session.ID,
		ActivityID:         session.GroupID,
		StartTime:          session.StartTime,
		LastActivity:       session.LastActivity,
		TimeoutMinutes:     session.TimeoutMinutes,
		InactivityDuration: session.GetInactivityDuration(),
		TimeUntilTimeout:   session.GetTimeUntilTimeout(),
		IsTimedOut:         session.IsTimedOut(),
		ActiveStudentCount: activeStudentCount,
	}

	return info, nil
}

// CleanupAbandonedSessions cleans up sessions that have been abandoned for longer than the specified duration.
// A session is considered abandoned if:
// 1. No activity (RFID scans or device pings) for longer than the threshold, AND
// 2. The device is offline (not pinging)
// This ensures sessions stay alive if either there's activity OR the device is still online.
func (s *service) CleanupAbandonedSessions(ctx context.Context, threshold time.Duration) (int, error) {
	// Find sessions with no activity since the threshold
	cutoffTime := time.Now().Add(-threshold)
	sessions, err := s.groupRepo.FindActiveSessionsOlderThan(ctx, cutoffTime)
	if err != nil {
		return 0, &ActiveError{Op: "CleanupAbandonedSessions", Err: err}
	}

	cleanedCount := 0
	for _, session := range sessions {
		// Session is abandoned only if BOTH conditions are true:
		// 1. No recent activity (already filtered by query)
		// 2. Device is offline (not pinging)
		deviceOnline := session.Device != nil && session.Device.IsOnline()
		if deviceOnline {
			// Device is still pinging - session stays alive
			continue
		}

		// Both conditions met: no activity AND device offline - clean up
		// Use ProcessSessionTimeoutByID with the session ID directly to prevent TOCTOU race condition
		// This ensures we end the exact session we identified as abandoned, not whatever
		// session happens to be current for the device at cleanup time
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

// EndDailySessions ends all active sessions at the end of the day
func (s *service) EndDailySessions(ctx context.Context) (*DailySessionCleanupResult, error) {
	result := &DailySessionCleanupResult{
		ExecutedAt: time.Now(),
		Success:    true,
		Errors:     make([]string, 0),
	}

	activeGroups, err := s.groupRepo.List(ctx, nil)
	if err != nil {
		result.Success = false
		return result, &ActiveError{Op: "EndDailySessions", Err: ErrDatabaseOperation}
	}

	for _, group := range activeGroups {
		if group.IsActive() {
			s.processGroupForDailyCleanup(ctx, group, result)
		}
	}

	// Clean up orphaned supervisors from previous days that the per-group loop wouldn't catch
	// (e.g., supervisors whose groups were already ended but end_date was never set)
	s.cleanupOrphanedSupervisors(ctx, result)

	return result, nil
}

// processGroupForDailyCleanup processes a single group for daily cleanup
func (s *service) processGroupForDailyCleanup(ctx context.Context, group *active.Group, result *DailySessionCleanupResult) {
	// Track error count before visit cleanup
	errorCountBefore := len(result.Errors)

	s.endActiveVisitsForGroup(ctx, group.ID, result)

	// If visit cleanup failed (e.g., database error fetching visits),
	// skip session and supervisor cleanup to maintain data consistency.
	// We don't want to end the session/supervisors while visits remain active.
	if len(result.Errors) > errorCountBefore {
		return
	}

	s.endGroupSession(ctx, group, result)
	s.endActiveSupervisorsForGroup(ctx, group.ID, result)
}

// endActiveVisitsForGroup ends all active visits for a group
func (s *service) endActiveVisitsForGroup(ctx context.Context, groupID int64, result *DailySessionCleanupResult) {
	visits, err := s.visitRepo.FindByActiveGroupID(ctx, groupID)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get visits for group %d: %v", groupID, err)
		result.Errors = append(result.Errors, errMsg)
		result.Success = false
		return
	}

	for _, visit := range visits {
		if !visit.IsActive() {
			continue
		}

		visit.EndVisit()
		if err := s.visitRepo.Update(ctx, visit); err != nil {
			errMsg := fmt.Sprintf("Failed to end visit %d: %v", visit.ID, err)
			result.Errors = append(result.Errors, errMsg)
			result.Success = false
		} else {
			result.VisitsEnded++
		}
	}
}

// endGroupSession ends a group session
func (s *service) endGroupSession(ctx context.Context, group *active.Group, result *DailySessionCleanupResult) {
	group.EndSession()
	if err := s.groupRepo.Update(ctx, group); err != nil {
		errMsg := fmt.Sprintf("Failed to end group session %d: %v", group.ID, err)
		result.Errors = append(result.Errors, errMsg)
		result.Success = false
	} else {
		result.SessionsEnded++
	}
}

// endActiveSupervisorsForGroup ends all active supervisors for a group
func (s *service) endActiveSupervisorsForGroup(ctx context.Context, groupID int64, result *DailySessionCleanupResult) {
	supervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, groupID, true)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get supervisors for group %d: %v", groupID, err)
		result.Errors = append(result.Errors, errMsg)
		result.Success = false
		return
	}

	now := time.Now()
	for _, supervisor := range supervisors {
		supervisor.EndDate = &now
		if err := s.supervisorRepo.Update(ctx, supervisor); err != nil {
			errMsg := fmt.Sprintf("Failed to end supervisor %d: %v", supervisor.ID, err)
			result.Errors = append(result.Errors, errMsg)
			result.Success = false
		} else {
			result.SupervisorsEnded++
		}
	}
}

// cleanupOrphanedSupervisors closes supervisor records from previous days
// that the per-group loop wouldn't find (e.g., groups already ended but supervisors left open)
func (s *service) cleanupOrphanedSupervisors(ctx context.Context, result *DailySessionCleanupResult) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Find orphaned supervisor records from before today with no end_date
	var staleRecords []struct {
		ID        int64     `bun:"id"`
		StartDate time.Time `bun:"start_date"`
	}

	err := s.db.NewSelect().
		Table("active.group_supervisors").
		Column("id", "start_date").
		Where("start_date < ?", today).
		Where("end_date IS NULL").
		Scan(ctx, &staleRecords)

	if err != nil {
		errMsg := fmt.Sprintf("Failed to find orphaned supervisors: %v", err)
		result.Errors = append(result.Errors, errMsg)
		result.Success = false
		return
	}

	for _, record := range staleRecords {
		// end_date is a DATE column, so set it to the start_date itself
		endDate := time.Date(
			record.StartDate.Year(), record.StartDate.Month(), record.StartDate.Day(),
			0, 0, 0, 0, record.StartDate.Location(),
		)

		_, err := s.db.NewUpdate().
			Table("active.group_supervisors").
			Set("end_date = ?", endDate).
			Set("updated_at = ?", now).
			Where("id = ?", record.ID).
			Exec(ctx)

		if err != nil {
			errMsg := fmt.Sprintf("Failed to close orphaned supervisor %d: %v", record.ID, err)
			result.Errors = append(result.Errors, errMsg)
			result.Success = false
		} else {
			result.SupervisorsEnded++
		}
	}
}
