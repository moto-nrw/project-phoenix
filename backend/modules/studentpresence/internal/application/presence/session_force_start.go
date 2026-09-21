package presence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ForceStartActivitySessionWithSupervisors starts an activity session with multiple supervisors and override capability
func (s *service) ForceStartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*ports.ActiveGroup, error) {
	s.getLogger().DebugContext(ctx, "force start with multiple supervisors called",
		slog.Any("supervisor_ids", supervisorIDs),
		slog.Int("supervisor_count", len(supervisorIDs)),
		slog.Int64("activity_id", activityID),
		slog.Int64("device_id", deviceID),
	)

	if err := s.validateSupervisorIDs(ctx, supervisorIDs); err != nil {
		return nil, err
	}

	var newGroup *ports.ActiveGroup
	err := s.forceStartActivitySessionTx(ctx, activityID, deviceID, supervisorIDs, roomID, &newGroup)
	if err != nil {
		return nil, err
	}

	return newGroup, nil
}

const opForceStartSession = "ForceStartActivitySessionWithSupervisors"

func (s *service) forceStartActivitySessionTx(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64, newGroup **ports.ActiveGroup) error {
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		lockedStaff, finalRoomID, err := s.lockForceStart(txCtx, activityID, deviceID, supervisorIDs, roomID)
		if err != nil {
			return err
		}

		conflictingSessionIDs, endedSessionIDs, err := s.endSessionsForForceStart(txCtx, activityID, deviceID)
		if err != nil {
			return err
		}
		if err := s.validateTransferredSupervisorLocks(txCtx, conflictingSessionIDs, lockedStaff); err != nil {
			return err
		}

		group, err := s.createSessionWithMultipleSupervisors(txCtx, activityID, deviceID, supervisorIDs, finalRoomID)
		if err != nil {
			return &ActiveError{Op: opForceStartSession, Err: err}
		}

		if err := s.transferForceStartedActivityState(txCtx, conflictingSessionIDs, group.ID, group.StartTime); err != nil {
			return &ActiveError{Op: opForceStartSession, Err: err}
		}

		if err := s.completeTimetableMirrorsForEndedSessions(txCtx, endedSessionIDs); err != nil {
			return &ActiveError{Op: opForceStartSession, Err: err}
		}

		*newGroup = group
		return nil
	})
	return markRollbackOnRoomCapacity(ctx, err)
}

// lockForceStart takes the force start's locks in their fixed order: the
// activity start lock, the requested supervisors, then the rooms involved. It
// returns the locked staff and the room the session starts in.
func (s *service) lockForceStart(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (map[int64]bool, int64, error) {
	if err := s.acquireActivitySessionLock(ctx, activityID, opForceStartSession); err != nil {
		return nil, 0, err
	}
	lockedStaff, err := s.lockForceStartSupervisors(ctx, activityID, supervisorIDs)
	if err != nil {
		return nil, 0, err
	}

	finalRoomID, err := s.determineRoomIDForForceStart(ctx, activityID, roomID)
	if err != nil {
		return nil, 0, &ActiveError{Op: opForceStartSession, Err: err}
	}
	if err := s.lockForceStartRooms(ctx, activityID, deviceID, finalRoomID); err != nil {
		return nil, 0, &ActiveError{Op: opForceStartSession, Err: ErrDatabaseOperation}
	}
	return lockedStaff, finalRoomID, nil
}

// endSessionsForForceStart ends the device's session and the activity's other
// sessions. It returns the activity sessions whose state moves to the new
// session and every session it ended.
func (s *service) endSessionsForForceStart(ctx context.Context, activityID, deviceID int64) (conflictingSessionIDs, endedSessionIDs []int64, err error) {
	// Use simple cleanup (fullCleanup=false) to only mark the group as ended
	// without ending visits, so TransferVisitsFromRecentSessions can move them
	// to the new session. Using fullCleanup=true would set exit_time on all visits
	// first, causing the transfer to find nothing and losing all checked-in students.
	deviceEndedSessionID, err := s.endExistingDeviceSessionForForceStart(ctx, deviceID)
	if err != nil {
		return nil, nil, &ActiveError{Op: opForceStartSession, Err: err}
	}

	conflictingSessionIDs, err = s.endExistingActivitySessionsForForceStart(ctx, activityID)
	if err != nil {
		return nil, nil, &ActiveError{Op: opForceStartSession, Err: err}
	}
	endedSessionIDs = appendActiveGroupID(nil, deviceEndedSessionID)
	endedSessionIDs = appendActiveGroupIDs(endedSessionIDs, conflictingSessionIDs...)
	return conflictingSessionIDs, endedSessionIDs, nil
}

func (s *service) lockForceStartSupervisors(ctx context.Context, activityID int64, requested []int64) (map[int64]bool, error) {
	staffIDs := deduplicateSupervisorIDs(requested)
	groups, err := s.GroupRepo.FindActiveByGroupID(ctx, activityID)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		if group == nil {
			continue
		}
		supervisors, err := s.SupervisorRepo.FindByActiveGroupID(ctx, group.ID, true)
		if err != nil {
			return nil, err
		}
		for _, supervisor := range supervisors {
			if supervisor != nil {
				staffIDs[supervisor.StaffID] = true
			}
		}
	}
	if err := s.lockSupervisorsForAssignment(ctx, slices.Collect(maps.Keys(staffIDs))); err != nil {
		return nil, err
	}
	return staffIDs, nil
}

// Group locks are now held. A supervisor added since discovery requires a
// fresh command, not an out-of-order staff lock while holding supervision rows.
func (s *service) validateTransferredSupervisorLocks(ctx context.Context, groupIDs []int64, lockedStaff map[int64]bool) error {
	for _, groupID := range groupIDs {
		supervisors, err := s.SupervisorRepo.FindByActiveGroupID(ctx, groupID, true)
		if err != nil {
			return err
		}
		for _, supervisor := range supervisors {
			if supervisor != nil && !lockedStaff[supervisor.StaffID] {
				return ErrSessionConflict
			}
		}
	}
	return nil
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
		// The Student Presence owner releases the group; its visits and
		// supervisors move to the new session afterwards (#2697).
		if err := s.SchoolPresence.EndGroup(ctx, session.ID, s.now()); err != nil {
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

	if err := s.SchoolPresence.EndGroup(ctx, existingSession.ID, s.now()); err != nil {
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
		count, err := s.SchoolPresence.TransferOpenVisits(ctx, oldGroupID, newGroupID)
		return int(count), err
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
		incoming, err := s.SchoolPresence.CountOpenVisitsInGroup(ctx, oldGroupID)
		if err != nil {
			return 0, err
		}
		if incoming > 0 {
			if err := s.ensureRoomCapacity(ctx, newGroup.RoomID, incoming); err != nil {
				return 0, err
			}
		}
	}
	count, err := s.SchoolPresence.TransferOpenVisits(ctx, oldGroupID, newGroupID)
	return int(count), err
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

		transferredSupervisor := &ports.GroupSupervisor{
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
	return s.determineRoomIDWithStrategy(ctx, activityID, roomID, RoomConflictWarn, false)
}

// determineRoomIDWithStrategy determines room ID with configurable conflict
// handling strategy. lockRoom serializes the room against concurrent session
// writes; a force start locks its rooms itself once the final room is known.
func (s *service) determineRoomIDWithStrategy(ctx context.Context, activityID int64, roomID *int64, strategy RoomConflictStrategy, lockRoom bool) (int64, error) {
	// Manual room selection has highest priority
	if roomID != nil && *roomID > 0 {
		return s.validateManualRoomSelection(ctx, *roomID, strategy, lockRoom)
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
func (s *service) validateManualRoomSelection(ctx context.Context, roomID int64, strategy RoomConflictStrategy, lockRoom bool) (int64, error) {
	if lockRoom && s.GroupRepo != nil {
		if err := s.SchoolPresence.LockRoomSessionWrites(ctx, roomID); err != nil {
			return 0, err
		}
	}
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

func (s *service) lockForceStartRooms(ctx context.Context, activityID, deviceID, finalRoomID int64) error {
	roomIDs := map[int64]struct{}{finalRoomID: {}}
	activitySessions, err := s.GroupRepo.FindActiveByGroupID(ctx, activityID)
	if err != nil {
		return err
	}
	for _, session := range activitySessions {
		if session != nil && session.RoomID > 0 {
			roomIDs[session.RoomID] = struct{}{}
		}
	}
	deviceSession, err := s.GroupRepo.FindActiveByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}
	if deviceSession != nil && deviceSession.RoomID > 0 {
		roomIDs[deviceSession.RoomID] = struct{}{}
	}

	ids := make([]int64, 0, len(roomIDs))
	for roomID := range roomIDs {
		ids = append(ids, roomID)
	}
	slices.Sort(ids)
	for _, roomID := range ids {
		if err := s.SchoolPresence.LockRoomSessionWrites(ctx, roomID); err != nil {
			return err
		}
	}
	return nil
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
