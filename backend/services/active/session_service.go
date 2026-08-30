package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Activity Session Management with Conflict Detection

// determineSessionRoomID determines the room for a session with conflict checking
func (s *service) determineSessionRoomID(ctx context.Context, activityID int64, roomID *int64) (int64, error) {
	return s.determineRoomIDWithStrategy(ctx, activityID, roomID, RoomConflictFail)
}

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

	s.broadcastWithLogging(ctx, activeGroupID, "", event, "activity_start")

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

	staffMap, err := s.StaffRepo.FindByIDs(ctx, idSlice)
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
// Uses PostgreSQL advisory locks to prevent race conditions when multiple requests try to start the same activity concurrently.
// Wraps all operations in a transaction (via TxHandler.RunInTx) so the advisory lock is always available.
// If a transaction already exists in context (e.g. from handler-level WithTenantTx), it is reused.
func (s *service) executeSessionStart(ctx context.Context, activityID, deviceID int64, roomID *int64, operation string, createSession func(context.Context, int64) (*active.Group, error)) error {
	txHandler := modelBase.NewTxHandler(s.DB)

	err := txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		if err := s.acquireActivitySessionLock(txCtx, tx, activityID, operation); err != nil {
			return err
		}

		// Check for conflicts inside the transaction with the lock held
		conflictInfo, err := s.CheckActivityConflict(txCtx, activityID, deviceID)
		if err != nil {
			return &ActiveError{Op: operation, Err: err}
		}
		if conflictInfo.HasConflict {
			return &ActiveError{Op: operation, Err: ErrSessionConflict}
		}

		finalRoomID, err := s.determineSessionRoomID(txCtx, activityID, roomID)
		if err != nil {
			return err
		}

		_, err = createSession(txCtx, finalRoomID)
		return err
	})
	return markRollbackOnRoomCapacity(ctx, err)
}

func (s *service) acquireActivitySessionLock(ctx context.Context, tx bun.Tx, activityID int64, operation string) error {
	// Acquire advisory lock on (tenant_id, activity_id) to serialize concurrent session starts.
	// The two-argument form scopes locks per tenant; PostgreSQL releases it when the tx ends.
	tenantID := tenant.FromContext(ctx)
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?, ?)", tenantID, activityID); err != nil {
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
	session, err := s.WorkSessionService.EnsureCheckedIn(ctx, staffID, active.WorkSessionSourceNFC)
	var plannedStart *PlannedStartNotReachedError
	if errors.As(err, &plannedStart) {
		s.getLogger().InfoContext(ctx, "NFC auto-check-in skipped: planned start not reached",
			slog.Int64("staff_id", staffID),
			slog.Int64("group_id", groupID),
			slog.String("planned_start_time", plannedStart.PlannedStartTime),
			slog.String("current_time", plannedStart.CurrentTime),
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

// createSessionBase creates a new active group session and transfers visits from recent sessions
func (s *service) createSessionBase(ctx context.Context, activityID, deviceID, roomID int64) (*active.Group, int, error) {
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

	// Auto-update device location to the room where the session is starting
	if deviceID > 0 {
		s.updateDeviceLocation(ctx, deviceID, roomID)
	}

	transferredCount, err := s.VisitRepo.TransferVisitsFromRecentSessions(ctx, newGroup.ID, deviceID)
	if err != nil {
		return nil, 0, err
	}
	if transferredCount > 0 {
		if err := s.ensureRoomCapacity(ctx, roomID, 0); err != nil {
			return nil, 0, err
		}
	}

	return newGroup, transferredCount, nil
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
	tx, ok := modelBase.TxFromContext(ctx)
	if !ok || tx == nil {
		if err := fn(); err != nil {
			logFailure(err)
		}
		return
	}

	savepointName := "sp_active_" + label
	if _, err := (*tx).ExecContext(ctx, "SAVEPOINT "+savepointName); err != nil {
		s.getLogger().WarnContext(ctx, "failed to create savepoint for best-effort operation",
			slog.String("operation", label),
			slog.String("error", err.Error()),
		)
		return
	}

	if err := fn(); err != nil {
		logFailure(err)
		if _, rollbackErr := (*tx).ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+savepointName); rollbackErr != nil {
			s.getLogger().WarnContext(ctx, "failed to rollback savepoint for best-effort operation",
				slog.String("operation", label),
				slog.String("error", rollbackErr.Error()),
			)
		}
		return
	}

	if _, err := (*tx).ExecContext(ctx, "RELEASE SAVEPOINT "+savepointName); err != nil {
		s.getLogger().WarnContext(ctx, "failed to release savepoint for best-effort operation",
			slog.String("operation", label),
			slog.String("error", err.Error()),
		)
	}
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
	err := s.forceStartActivitySessionTx(ctx, activityID, deviceID, supervisorIDs, roomID, &newGroup)
	if err != nil {
		return nil, err
	}

	return newGroup, nil
}

func (s *service) forceStartActivitySessionTx(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64, newGroup **active.Group) error {
	const operation = "ForceStartActivitySessionWithSupervisors"
	txHandler := modelBase.NewTxHandler(s.DB)

	err := txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		if err := s.acquireActivitySessionLock(txCtx, tx, activityID, operation); err != nil {
			return err
		}

		// Use simple cleanup (fullCleanup=false) to only mark the group as ended
		// without ending visits, so TransferVisitsFromRecentSessions can move them
		// to the new session. Using fullCleanup=true would set exit_time on all visits
		// first, causing the transfer to find nothing and losing all checked-in students.
		deviceEndedSessionID, err := s.endExistingDeviceSessionForForceStart(txCtx, deviceID)
		if err != nil {
			return &ActiveError{Op: operation, Err: err}
		}

		conflictingSessionIDs, err := s.endExistingActivitySessionsForForceStart(txCtx, activityID)
		if err != nil {
			return &ActiveError{Op: operation, Err: err}
		}
		endedSessionIDs := appendActiveGroupID(nil, deviceEndedSessionID)
		endedSessionIDs = appendActiveGroupIDs(endedSessionIDs, conflictingSessionIDs...)

		finalRoomID, err := s.determineRoomIDForForceStart(txCtx, activityID, roomID)
		if err != nil {
			return &ActiveError{Op: operation, Err: err}
		}

		group, err := s.createSessionWithMultipleSupervisors(txCtx, activityID, deviceID, supervisorIDs, finalRoomID)
		if err != nil {
			return &ActiveError{Op: operation, Err: err}
		}

		if err := s.transferForceStartedActivityState(txCtx, conflictingSessionIDs, group.ID, group.StartTime); err != nil {
			return &ActiveError{Op: operation, Err: err}
		}

		if err := s.completeTimetableMirrorsForEndedSessions(txCtx, endedSessionIDs); err != nil {
			return &ActiveError{Op: operation, Err: err}
		}

		*newGroup = group
		return nil
	})
	return markRollbackOnRoomCapacity(ctx, err)
}

func markRollbackOnRoomCapacity(ctx context.Context, err error) error {
	if errors.Is(err, ErrRoomCapacityExceeded) {
		tenant.MarkRollback(ctx)
	}
	return err
}

func appendActiveGroupID(ids []int64, id int64) []int64 {
	if id <= 0 {
		return ids
	}
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

func appendActiveGroupIDs(ids []int64, more ...int64) []int64 {
	for _, id := range more {
		ids = appendActiveGroupID(ids, id)
	}
	return ids
}

// completeTimetableMirrorsForEndedSessions closes the schedule side of every
// active.group this service just ended: the bridge finalizes the attendance
// (expected → absent, non-bookings spared per #1747) and only then stamps the
// instance completed.
//
// Every path that ends a session has to come through here — force-start,
// kiosk timeout, the abandoned-session sweep and the manual group end alike. A
// session that ends without it leaves the mirrored instance active forever with
// its expected rows unfinalized, and nothing downstream ever repairs that.
func (s *service) completeTimetableMirrorsForEndedSessions(ctx context.Context, endedSessionIDs []int64) error {
	if s.TimetableBridgeCompleter == nil || len(endedSessionIDs) == 0 {
		return nil
	}
	completed, err := s.TimetableBridgeCompleter.CompleteActiveByActiveGroupIDs(ctx, endedSessionIDs, time.Now())
	if err != nil {
		return fmt.Errorf("complete timetable mirrors for ended sessions: %w", err)
	}
	if completed > 0 {
		s.getLogger().InfoContext(ctx, "completed timetable mirrors for ended activity sessions",
			slog.Int64("count", completed),
		)
	}
	return nil
}

func (s *service) endExistingActivitySessionsForForceStart(ctx context.Context, activityID int64) ([]int64, error) {
	existingSessions, err := s.GroupRepo.FindActiveByGroupID(ctx, activityID)
	if err != nil {
		return nil, err
	}

	endedIDs := make([]int64, 0, len(existingSessions))
	for _, session := range existingSessions {
		if session == nil || session.ID <= 0 {
			continue
		}
		locked, err := s.GroupRepo.FindByIDForUpdate(ctx, session.ID)
		if err != nil {
			return nil, err
		}
		if locked == nil || !locked.IsActive() {
			continue
		}
		if err := s.GroupRepo.EndSession(ctx, session.ID); err != nil {
			return nil, err
		}
		endedIDs = append(endedIDs, session.ID)
	}

	return endedIDs, nil
}

func (s *service) endExistingDeviceSessionForForceStart(ctx context.Context, deviceID int64) (int64, error) {
	existingSession, err := s.GroupRepo.FindActiveByDeviceID(ctx, deviceID)
	if err != nil {
		return 0, err
	}

	if existingSession == nil {
		return 0, nil
	}
	locked, err := s.GroupRepo.FindByIDForUpdate(ctx, existingSession.ID)
	if err != nil {
		return 0, err
	}
	if locked == nil || !locked.IsActive() {
		return 0, nil
	}

	if err := s.GroupRepo.EndSession(ctx, existingSession.ID); err != nil {
		return 0, err
	}

	return existingSession.ID, nil
}

func (s *service) transferForceStartedActivityState(ctx context.Context, oldGroupIDs []int64, newGroupID int64, newGroupStartTime time.Time) error {
	for _, oldGroupID := range oldGroupIDs {
		visitsTransferred, err := s.transferActiveVisitsBetweenGroups(ctx, oldGroupID, newGroupID)
		if err != nil {
			return err
		}

		supervisorsTransferred, err := s.transferActiveSupervisorsBetweenGroups(ctx, oldGroupID, newGroupID, newGroupStartTime)
		if err != nil {
			return err
		}

		if visitsTransferred > 0 || supervisorsTransferred > 0 {
			s.getLogger().InfoContext(ctx, "transferred state from force-ended activity session",
				slog.Int64("old_active_group_id", oldGroupID),
				slog.Int64("new_active_group_id", newGroupID),
				slog.Int("visits_transferred", visitsTransferred),
				slog.Int("supervisors_transferred", supervisorsTransferred),
			)
		}
	}

	return nil
}

func (s *service) transferActiveVisitsBetweenGroups(ctx context.Context, oldGroupID, newGroupID int64) (int, error) {
	if s.GroupRepo == nil {
		return s.VisitRepo.TransferActiveVisitsBetweenGroups(ctx, oldGroupID, newGroupID)
	}
	oldGroup, err := s.GroupRepo.FindByID(ctx, oldGroupID)
	if err != nil {
		return 0, err
	}
	newGroup, err := s.GroupRepo.FindByID(ctx, newGroupID)
	if err != nil {
		return 0, err
	}
	if oldGroup != nil && newGroup != nil && oldGroup.RoomID != newGroup.RoomID {
		incoming, err := s.VisitRepo.CountActiveByGroupID(ctx, oldGroupID)
		if err != nil {
			return 0, err
		}
		if incoming > 0 {
			if err := s.ensureRoomCapacity(ctx, newGroup.RoomID, incoming); err != nil {
				return 0, err
			}
		}
	}
	return s.VisitRepo.TransferActiveVisitsBetweenGroups(ctx, oldGroupID, newGroupID)
}

func (s *service) transferActiveSupervisorsBetweenGroups(ctx context.Context, oldGroupID, newGroupID int64, newGroupStartTime time.Time) (int, error) {
	oldSupervisors, err := s.SupervisorRepo.FindByActiveGroupID(ctx, oldGroupID, true)
	if err != nil {
		return 0, err
	}

	newSupervisors, err := s.SupervisorRepo.FindByActiveGroupID(ctx, newGroupID, true)
	if err != nil {
		return 0, err
	}

	existingStaffIDs := make(map[int64]bool, len(newSupervisors))
	for _, supervisor := range newSupervisors {
		if supervisor == nil {
			continue
		}
		existingStaffIDs[supervisor.StaffID] = true
	}

	transferred := 0
	for _, supervisor := range oldSupervisors {
		if supervisor == nil {
			continue
		}

		if err := s.SupervisorRepo.EndSupervision(ctx, supervisor.ID); err != nil {
			return transferred, err
		}

		if existingStaffIDs[supervisor.StaffID] {
			continue
		}

		transferredSupervisor := &active.GroupSupervisor{
			StaffID:   supervisor.StaffID,
			GroupID:   newGroupID,
			Role:      normalizeTransferredSupervisorRole(supervisor.Role),
			StartDate: timezone.DateFromTime(newGroupStartTime),
		}
		transferredSupervisor.SetTenantID(tenant.FromContext(ctx))
		if err := s.SupervisorRepo.Create(ctx, transferredSupervisor); err != nil {
			return transferred, err
		}
		existingStaffIDs[supervisor.StaffID] = true
		transferred++
	}

	return transferred, nil
}

func normalizeTransferredSupervisorRole(role string) string {
	if strings.EqualFold(role, "supervisor") {
		return "supervisor"
	}
	return role
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

	// Try to get planned room from activity configuration.
	plannedRoomID, err := s.getPlannedRoomID(ctx, activityID)
	if err != nil {
		return 0, err
	}
	if plannedRoomID > 0 {
		return plannedRoomID, nil
	}

	// No room selected and none planned: there is no tenant-safe default.
	return 0, ErrNoRoomAvailable
}

// validateManualRoomSelection validates manually selected room based on conflict strategy
func (s *service) validateManualRoomSelection(ctx context.Context, roomID int64, strategy RoomConflictStrategy) (int64, error) {
	if strategy == RoomConflictIgnore {
		return roomID, nil
	}

	hasConflict, _, err := s.GroupRepo.CheckRoomConflict(ctx, roomID, 0)
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

// getPlannedRoomID retrieves the planned room ID from activity configuration.
// A missing planned room is distinct from a repository error: callers return
// ErrNoRoomAvailable only for the former and preserve the latter's 5xx path.
func (s *service) getPlannedRoomID(ctx context.Context, activityID int64) (int64, error) {
	activityGroup, err := s.ActivityGroupRepo.FindByID(ctx, activityID)
	if err != nil {
		return 0, err
	}
	if activityGroup != nil && activityGroup.PlannedRoomID != nil && *activityGroup.PlannedRoomID > 0 {
		return *activityGroup.PlannedRoomID, nil
	}
	return 0, nil
}

// UpdateActiveGroupSupervisors replaces all supervisors for an active group
func (s *service) UpdateActiveGroupSupervisors(ctx context.Context, activeGroupID int64, supervisorIDs []int64) (*active.Group, error) {
	if err := s.validateSupervisorIDs(ctx, supervisorIDs); err != nil {
		return nil, err
	}

	uniqueSupervisors := deduplicateSupervisorIDs(supervisorIDs)
	if err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		if err := s.lockActiveGroupForSupervisorUpdate(txCtx, activeGroupID); err != nil {
			return err
		}
		return s.replaceSupervisorsInTransaction(txCtx, activeGroupID, uniqueSupervisors)
	}); err != nil {
		return nil, &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: err}
	}

	// Supervisor takeover/handover means these staff members are on site —
	// auto-open their work sessions so they show as "Anwesend" (issue #1439).
	// Same best-effort semantics as the session-start auto-stamp.
	source := active.WorkSessionSourceApp
	if device.IsIoTDeviceRequest(ctx) {
		source = active.WorkSessionSourceNFC
	}
	for staffID := range uniqueSupervisors {
		s.ensureStaffPresence(ctx, staffID, source)
	}

	updatedGroup, err := s.GroupRepo.FindWithSupervisors(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: err}
	}

	return updatedGroup, nil
}

// validateActiveGroupForSupervisorUpdate validates that the group exists and is active
func (s *service) lockActiveGroupForSupervisorUpdate(ctx context.Context, activeGroupID int64) error {
	activeGroup, err := s.GroupRepo.FindByIDForUpdate(ctx, activeGroupID)
	if err != nil || activeGroup == nil {
		return &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: ErrActiveGroupNotFound}
	}

	if !activeGroup.IsActive() {
		return &ActiveError{Op: "UpdateActiveGroupSupervisors", Err: fmt.Errorf("cannot update supervisors for an ended session")}
	}

	return nil
}

func (s *service) lockGroupRows(ctx context.Context, groupIDs ...int64) error {
	unique := make(map[int64]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		if id > 0 {
			unique[id] = struct{}{}
		}
	}
	ids := slices.Collect(maps.Keys(unique))
	slices.Sort(ids)
	for _, id := range ids {
		group, err := s.GroupRepo.FindByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if group == nil {
			return ErrActiveGroupNotFound
		}
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
	currentSupervisors, err := s.SupervisorRepo.FindByActiveGroupID(ctx, activeGroupID, true)
	if err != nil {
		return err
	}

	uniqueSupervisors = primarySupervisorIDs(uniqueSupervisors, currentSupervisors)
	if err := s.endAllCurrentSupervisors(ctx, currentSupervisors); err != nil {
		return err
	}

	return s.upsertSupervisors(ctx, activeGroupID, uniqueSupervisors, currentSupervisors)
}

// primarySupervisorIDs excludes non-primary roles from a primary-supervisor
// replacement. IoT check-ins pass every active supervisor ID, including
// additional supervisors, which must remain assigned to the session.
func primarySupervisorIDs(supervisorIDs map[int64]bool, currentSupervisors []*active.GroupSupervisor) map[int64]bool {
	primaryIDs := maps.Clone(supervisorIDs)
	for _, supervisor := range currentSupervisors {
		if supervisor.Role != "supervisor" {
			delete(primaryIDs, supervisor.StaffID)
		}
	}
	return primaryIDs
}

// endAllCurrentSupervisors ends the current primary supervisors by setting end_date.
func (s *service) endAllCurrentSupervisors(ctx context.Context, supervisors []*active.GroupSupervisor) error {
	today := timezone.TodayDate()
	for _, supervisor := range supervisors {
		if supervisor.Role != "supervisor" {
			continue
		}
		supervisor.EndDate = &today
		if err := s.SupervisorRepo.Update(ctx, supervisor); err != nil {
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
	supervisor.StartDate = timezone.DateFromTime(now)
	return s.SupervisorRepo.Update(ctx, supervisor)
}

// createNewSupervisor creates a new supervisor record
func (s *service) createNewSupervisor(ctx context.Context, activeGroupID, supervisorID int64, now time.Time) error {
	supervisor := &active.GroupSupervisor{
		StaffID:   supervisorID,
		GroupID:   activeGroupID,
		Role:      "supervisor",
		StartDate: timezone.DateFromTime(now),
	}
	supervisor.SetTenantID(tenant.FromContext(ctx))
	return s.SupervisorRepo.Create(ctx, supervisor)
}

// CheckActivityConflict checks for conflicts before starting an activity session
func (s *service) CheckActivityConflict(ctx context.Context, activityID, deviceID int64) (*ActivityConflictInfo, error) {
	// Check if device is already running another session
	existingDeviceSession, err := s.GroupRepo.FindActiveByDeviceID(ctx, deviceID)
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
	existingActivitySessions, err := s.GroupRepo.FindActiveByGroupID(ctx, activityID)
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
	var visitsToNotify []visitSSEData
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		group, err := s.GroupRepo.FindByIDForUpdate(txCtx, activeGroupID)
		if err != nil || group == nil {
			return &ActiveError{Op: "EndActivitySession", Err: ErrActiveGroupNotFound}
		}
		if !group.IsActive() {
			return &ActiveError{Op: "EndActivitySession", Err: ErrActiveGroupAlreadyEnded}
		}
		visitsToNotify, err = s.endActivitySessionLocked(txCtx, activeGroupID)
		return err
	})
	if err != nil {
		return err
	}
	s.queueActivitySessionEndBroadcasts(ctx, activeGroupID, visitsToNotify)
	return nil
}

func (s *service) endActivitySessionLocked(ctx context.Context, activeGroupID int64) ([]visitSSEData, error) {
	// Collect active visits before mutating them for the SSE payloads.
	visitsToNotify, err := s.collectActiveVisitsForSSE(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: "EndActivitySession", Err: ErrDatabaseOperation}
	}

	// End all active visits
	for _, visitData := range visitsToNotify {
		if _, _, err := s.endVisitWithAttendanceSync(ctx, visitData.VisitID); err != nil {
			return nil, &ActiveError{Op: "EndActivitySession", Err: err}
		}
	}

	// End all active supervisors
	if err := s.endActiveSupervisors(ctx, activeGroupID); err != nil {
		return nil, err
	}

	// End the session
	if err := s.GroupRepo.EndSession(ctx, activeGroupID); err != nil {
		return nil, &ActiveError{Op: "EndActivitySession", Err: err}
	}
	return visitsToNotify, nil
}

func (s *service) endActiveSupervisors(ctx context.Context, activeGroupID int64) error {
	activeSupervisors, err := s.SupervisorRepo.FindByActiveGroupID(ctx, activeGroupID, true)
	if err != nil {
		return &ActiveError{Op: "EndActivitySession", Err: err}
	}
	for _, supervisor := range activeSupervisors {
		if err := s.SupervisorRepo.EndSupervision(ctx, supervisor.ID); err != nil {
			return &ActiveError{Op: "EndActivitySession", Err: err}
		}
	}
	return nil
}

func (s *service) queueActivitySessionEndBroadcasts(ctx context.Context, activeGroupID int64, visits []visitSSEData) {
	if s.Broadcaster == nil {
		return
	}
	broadcastCtx := modelBase.ContextWithoutTx(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		activeGroupIDStr := fmt.Sprintf("%d", activeGroupID)
		s.broadcastStudentCheckoutEvents(broadcastCtx, activeGroupIDStr, visits)
		s.broadcastActivityEndEvent(broadcastCtx, activeGroupID, activeGroupIDStr)
	})
}

// GetDeviceCurrentSession gets the current active session for a device
func (s *service) GetDeviceCurrentSession(ctx context.Context, deviceID int64) (*active.Group, error) {
	session, err := s.GroupRepo.FindActiveByDeviceIDWithNames(ctx, deviceID)
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

// checkoutActiveVisits ends all active visits for a session and returns the count of students checked out.
func (s *service) checkoutActiveVisits(ctx context.Context, sessionID int64) (int, error) {
	visits, err := s.VisitRepo.FindByActiveGroupID(ctx, sessionID)
	if err != nil {
		return 0, err
	}

	studentsCheckedOut := 0
	for _, visit := range visits {
		if !visit.IsActive() {
			continue
		}
		if _, _, err := s.endVisitWithAttendanceSync(ctx, visit.ID); err != nil {
			return 0, err
		}
		studentsCheckedOut++
	}

	return studentsCheckedOut, nil
}

// collectActiveVisitsForSSE gathers visit and student data needed for SSE broadcasts
func (s *service) collectActiveVisitsForSSE(ctx context.Context, sessionID int64) ([]visitSSEData, error) {
	visits, err := s.VisitRepo.FindByActiveGroupID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Filter active visits and collect unique student IDs
	var activeVisits []*active.Visit
	studentIDSet := make(map[int64]struct{})
	for _, visit := range visits {
		if !visit.IsActive() {
			continue
		}
		activeVisits = append(activeVisits, visit)
		studentIDSet[visit.StudentID] = struct{}{}
	}

	if len(activeVisits) == 0 {
		return nil, nil
	}

	// Batch-fetch all students (1 query instead of N)
	studentIDs := slices.Collect(maps.Keys(studentIDSet))
	studentsMap, err := s.StudentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		studentsMap = nil
	}

	// Build result using map lookups (O(1) per visit)
	result := make([]visitSSEData, 0, len(activeVisits))
	for _, visit := range activeVisits {
		data := visitSSEData{
			VisitID:   visit.ID,
			StudentID: visit.StudentID,
		}
		if student, ok := studentsMap[visit.StudentID]; ok && student != nil {
			data.Student = student
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

	// Every write below runs in ONE transaction (#1747 review). The abandoned-
	// session sweep calls this straight from the scheduler, with no request
	// middleware around it: without a transaction of its own the bridge would
	// commit a completed timetable instance and a later failure would leave the
	// session open beside it — a split that the next sweep cannot repair,
	// because it only ever sees the still-active session and would re-bridge an
	// instance that is already completed. RunInTx joins the caller's
	// transaction when there is one (the kiosk timeout endpoint), so the
	// request path is unchanged.
	var result *TimeoutResult
	if err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		res, err := s.processSessionTimeoutTx(txCtx, sessionID)
		if err != nil {
			return err
		}
		result = res
		return nil
	}); err != nil {
		if activeErr, ok := err.(*ActiveError); ok {
			return nil, activeErr
		}
		return nil, &ActiveError{Op: "ProcessSessionTimeoutByID", Err: err}
	}

	// Broadcast SSE events (fire-and-forget, outside transaction)
	if s.Broadcaster != nil && result != nil {
		sessionIDStr := fmt.Sprintf("%d", sessionID)
		s.broadcastStudentCheckoutEvents(ctx, sessionIDStr, visitsToNotify)
		s.broadcastActivityEndEvent(ctx, sessionID, sessionIDStr)
	}

	return result, nil
}

// processSessionTimeoutTx holds every write of a session timeout. It runs
// inside one transaction and announces nothing — the SSE events belong to the
// caller, after the commit.
func (s *service) processSessionTimeoutTx(ctx context.Context, sessionID int64) (*TimeoutResult, error) {
	session, err := s.GroupRepo.FindByIDForUpdate(ctx, sessionID)
	if err != nil || session == nil {
		return nil, &ActiveError{Op: "ProcessSessionTimeoutByID", Err: ErrActiveGroupNotFound}
	}
	if !session.IsActive() {
		return nil, &ActiveError{Op: "ProcessSessionTimeoutByID", Err: ErrActiveGroupAlreadyEnded}
	}

	// The timetable side closes FIRST, before anything is announced (#1747
	// review). A timed-out session is still a session end: without the bridge
	// the mirrored instance stays active indefinitely and its expected rows are
	// never finalized. Running it ahead of the checkouts and the session end
	// keeps the failure-prone half in front of the SSE events the caller fires,
	// so a bridge error never announces an end that the transaction rolls back.
	if err := s.completeTimetableMirrorsForEndedSessions(ctx, []int64{sessionID}); err != nil {
		return nil, &ActiveError{Op: "ProcessSessionTimeoutByID", Err: err}
	}

	studentsCheckedOut, err := s.checkoutActiveVisits(ctx, sessionID)
	if err != nil {
		return nil, &ActiveError{Op: "ProcessSessionTimeoutByID", Err: err}
	}
	if err := s.endActiveSupervisors(ctx, sessionID); err != nil {
		return nil, err
	}

	if err := s.GroupRepo.EndSession(ctx, sessionID); err != nil {
		return nil, &ActiveError{Op: "ProcessSessionTimeoutByID", Err: err}
	}

	return &TimeoutResult{
		SessionID:          sessionID,
		ActivityID:         session.GroupID,
		StudentsCheckedOut: studentsCheckedOut,
		TimeoutAt:          time.Now(),
	}, nil
}

// runInSessionTx runs fn inside a transaction, joining the caller's when one is
// already in context. Without a database handle — the shape unit tests build,
// where every repository is a double and there is nothing to commit — fn runs
// directly; production wiring always supplies one via services.NewFactory.
func (s *service) runInSessionTx(ctx context.Context, fn func(context.Context) error) error {
	if s.DB == nil {
		return fn(ctx)
	}
	return modelBase.NewTxHandler(s.DB).RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

// UpdateSessionActivity updates the last activity timestamp for a session
func (s *service) UpdateSessionActivity(ctx context.Context, activeGroupID int64) error {
	if err := s.GroupRepo.UpdateLastActivity(ctx, activeGroupID, time.Now()); err != nil {
		if isUpdateLastActivitySessionMiss(err) {
			session, findErr := s.GroupRepo.FindByID(ctx, activeGroupID)
			if findErr != nil {
				if isFindByIDNoRows(findErr) {
					return &ActiveError{Op: "UpdateSessionActivity", Err: ErrActiveGroupNotFound}
				}
				return &ActiveError{Op: "UpdateSessionActivity", Err: findErr}
			}

			if session == nil {
				return &ActiveError{Op: "UpdateSessionActivity", Err: ErrActiveGroupNotFound}
			}

			if !session.IsActive() {
				return &ActiveError{Op: "UpdateSessionActivity", Err: ErrActiveGroupAlreadyEnded}
			}
		}

		return &ActiveError{Op: "UpdateSessionActivity", Err: err}
	}
	return nil
}

func isUpdateLastActivitySessionMiss(err error) bool {
	var dbErr *modelBase.DatabaseError
	return errors.As(err, &dbErr) && dbErr.Op == "update last activity - session not found"
}

func isFindByIDNoRows(err error) bool {
	var dbErr *modelBase.DatabaseError
	return errors.As(err, &dbErr) && dbErr.Op == "find by id" && errors.Is(dbErr.Err, sql.ErrNoRows)
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
	visits, err := s.VisitRepo.FindByActiveGroupID(ctx, session.ID)
	if err != nil {
		return nil, &ActiveError{Op: "GetSessionTimeoutInfo", Err: err}
	}

	activeStudentCount := 0
	for _, visit := range visits {
		if visit.IsActive() {
			activeStudentCount++
		}
	}

	now := time.Now()
	info := &SessionTimeoutInfo{
		SessionID:          session.ID,
		ActivityID:         session.GroupID,
		StartTime:          session.StartTime,
		LastActivity:       session.LastActivity,
		TimeoutMinutes:     session.TimeoutMinutes,
		InactivityDuration: SessionInactivityDuration(session, now),
		TimeUntilTimeout:   s.SessionTimeUntilTimeout(ctx, session, now),
		IsTimedOut:         s.IsSessionTimedOut(ctx, session, now),
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
		// Stamp the session's tenant into the context: the CLI cleanup path
		// (cmd/cleanup.go) calls with context.Background(), and the SSE
		// broadcasts inside the timeout flow are tenant-scoped
		// (BroadcastToTenant) — without a tenant id they would be silently
		// dropped. The scheduler path already carries the same tenant id, so
		// re-stamping is a no-op there.
		sessionCtx := tenant.WithTenantID(ctx, session.TenantID)
		_, err := s.ProcessSessionTimeoutByID(sessionCtx, session.ID)
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
func (s *service) isDeviceOnline(ctx context.Context, device *iotModels.Device, now time.Time) bool {
	if device == nil || device.LastSeen == nil {
		return false
	}
	return now.Sub(*device.LastSeen) <= s.deviceOnlineWindow(ctx)
}

// deviceOnlineWindow resolves the per-tenant device-online window, falling back
// to defaultDeviceOnlineWindow.
func (s *service) deviceOnlineWindow(ctx context.Context) time.Duration {
	minutes := config.ResolveIntOrDefault(ctx, s.settings, configModel.KeyDeviceOnlineWindowMinutes, 0, s.getLogger())
	if minutes <= 0 {
		return defaultDeviceOnlineWindow
	}
	return time.Duration(minutes) * time.Minute
}

// EndDailySessions ends all active sessions at the end of the day using bulk UPDATEs
func (s *service) EndDailySessions(ctx context.Context) (*DailySessionCleanupResult, error) {
	// Binary-mode tenants don't open activity sessions or visits (L3.1 gates
	// CreateVisit/EndVisit to no-ops), so this job has nothing to close for
	// them. Returning early saves a handful of per-tenant queries on every
	// scheduler tick and keeps the result shape unchanged for callers.
	if s.GetPresenceMode(ctx) == "binary" {
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
	activeGroups, err := s.GroupRepo.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	activeIDs := make([]int64, 0, len(activeGroups))
	for _, group := range activeGroups {
		if group.IsActive() {
			activeIDs = append(activeIDs, group.ID)
		}
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

	// 2. Bulk end visits — abort remaining steps on failure to prevent
	// sessions/supervisors being closed while visits remain active.
	visitsEnded, err := s.VisitRepo.EndVisitsByActiveGroupIDs(ctx, activeIDs)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to bulk-end visits: %v", err))
		result.Success = false
		return err
	}
	result.VisitsEnded = int(visitsEnded)

	// 3. Bulk end sessions
	sessionsEnded, err := s.GroupRepo.EndSessionsByIDs(ctx, activeIDs)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to bulk-end sessions: %v", err))
		result.Success = false
		return err
	}
	result.SessionsEnded = int(sessionsEnded)
	result.EndedActiveGroupIDs = append(result.EndedActiveGroupIDs, activeIDs...)

	// 4. Bulk end supervisors
	supervisorsEnded, err := s.SupervisorRepo.EndSupervisionsByActiveGroupIDs(ctx, activeIDs)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to bulk-end supervisors: %v", err))
		result.Success = false
		return err
	}
	result.SupervisorsEnded = int(supervisorsEnded)

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

func (s *service) closeStaleSupervisor(ctx context.Context, record *active.GroupSupervisor, today timezone.Date) (bool, error) {
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
		_, err = s.SupervisorRepo.UpdateColumns(txCtx, current, "end_date", "updated_at")
		closed = err == nil
		return err
	})
	return closed, err
}
