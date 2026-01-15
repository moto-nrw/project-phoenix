package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/uptrace/bun"
)

// Session Start Management
// This file contains methods for starting activity sessions with conflict detection,
// supervisor assignment, and room management.

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

// ForceStartActivitySessionWithSupervisors starts an activity session with multiple supervisors and override capability
func (s *service) ForceStartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*active.Group, error) {
	if err := s.validateSupervisorIDs(ctx, supervisorIDs); err != nil {
		return nil, err
	}

	var newGroup *active.Group
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		if err := s.endExistingDeviceSessionWithCleanup(ctx, deviceID); err != nil {
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

// executeSessionStart handles common session start logic: conflict checking, device validation, and room determination
func (s *service) executeSessionStart(ctx context.Context, activityID, deviceID int64, roomID *int64, operation string, createSession func(context.Context, int64) (*active.Group, error)) error {
	conflictInfo, err := s.CheckActivityConflict(ctx, activityID, deviceID)
	if err != nil {
		return &ActiveError{Op: operation, Err: err}
	}
	if conflictInfo.HasConflict {
		return &ActiveError{Op: operation, Err: ErrSessionConflict}
	}

	return s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		existingSession, err := s.groupRepo.FindActiveByDeviceID(ctx, deviceID)
		if err != nil {
			return err
		}
		if existingSession != nil {
			return ErrDeviceAlreadyActive
		}

		finalRoomID, err := s.determineSessionRoomID(ctx, activityID, roomID)
		if err != nil {
			return err
		}

		_, err = createSession(ctx, finalRoomID)
		return err
	})
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

// createSessionWithSupervisor creates a new session, assigns supervisor, and transfers visits
func (s *service) createSessionWithSupervisor(ctx context.Context, activityID, deviceID, staffID, roomID int64) (*active.Group, error) {
	newGroup, transferredCount, err := s.createSessionBase(ctx, activityID, deviceID, roomID)
	if err != nil {
		return nil, err
	}

	s.assignSupervisorNonCritical(ctx, newGroup.ID, staffID, newGroup.StartTime)

	if transferredCount > 0 {
		fmt.Printf(visitTransferMessage, transferredCount, newGroup.ID)
	}

	return newGroup, nil
}

// createSessionWithMultipleSupervisors creates a new session with multiple supervisors and transfers visits
func (s *service) createSessionWithMultipleSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID int64) (*active.Group, error) {
	newGroup, transferredCount, err := s.createSessionBase(ctx, activityID, deviceID, roomID)
	if err != nil {
		return nil, err
	}

	s.assignMultipleSupervisorsNonCritical(ctx, newGroup.ID, supervisorIDs, newGroup.StartTime)

	if transferredCount > 0 {
		fmt.Printf(visitTransferMessage, transferredCount, newGroup.ID)
	}

	return newGroup, nil
}

// createSessionWithSupervisorForceStart creates a session for force start with special logging
func (s *service) createSessionWithSupervisorForceStart(ctx context.Context, activityID, deviceID, staffID, roomID int64) (*active.Group, error) {
	newGroup, transferredCount, err := s.createSessionBase(ctx, activityID, deviceID, roomID)
	if err != nil {
		return nil, err
	}

	s.assignSupervisorNonCritical(ctx, newGroup.ID, staffID, newGroup.StartTime)

	if transferredCount > 0 {
		fmt.Printf("Transferred %d active visits to new session %d (force start)\n", transferredCount, newGroup.ID)
	}

	return newGroup, nil
}

// assignSupervisorNonCritical assigns a supervisor but doesn't fail if assignment fails
func (s *service) assignSupervisorNonCritical(ctx context.Context, groupID, staffID int64, startDate time.Time) {
	supervisor := &active.GroupSupervisor{
		StaffID:   staffID,
		GroupID:   groupID,
		Role:      "Supervisor",
		StartDate: startDate,
	}
	if err := s.supervisorRepo.Create(ctx, supervisor); err != nil {
		fmt.Printf(supervisorAssignmentWarning, staffID, groupID, err)
	}
}

// assignMultipleSupervisorsNonCritical assigns multiple supervisors but doesn't fail if assignment fails
func (s *service) assignMultipleSupervisorsNonCritical(ctx context.Context, groupID int64, supervisorIDs []int64, startDate time.Time) {
	// Deduplicate supervisor IDs
	uniqueSupervisors := make(map[int64]bool)
	for _, id := range supervisorIDs {
		uniqueSupervisors[id] = true
	}

	// Assign each unique supervisor
	for staffID := range uniqueSupervisors {
		supervisor := &active.GroupSupervisor{
			StaffID:   staffID,
			GroupID:   groupID,
			Role:      "supervisor",
			StartDate: startDate,
		}
		if err := s.supervisorRepo.Create(ctx, supervisor); err != nil {
			fmt.Printf(supervisorAssignmentWarning, staffID, groupID, err)
		}
	}
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

// endExistingDeviceSessionIfPresent ends any existing session for the device using simple cleanup
func (s *service) endExistingDeviceSessionIfPresent(ctx context.Context, deviceID int64) error {
	return s.endExistingDeviceSession(ctx, deviceID, false)
}

// endExistingDeviceSessionWithCleanup ends existing device session using full cleanup (EndActivitySession)
func (s *service) endExistingDeviceSessionWithCleanup(ctx context.Context, deviceID int64) error {
	return s.endExistingDeviceSession(ctx, deviceID, true)
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

// determineSessionRoomID determines the room for a session with conflict checking
func (s *service) determineSessionRoomID(ctx context.Context, activityID int64, roomID *int64) (int64, error) {
	return s.determineRoomIDWithStrategy(ctx, activityID, roomID, RoomConflictFail)
}

// determineRoomIDWithoutConflictCheck determines room ID without checking conflicts (for force start)
func (s *service) determineRoomIDWithoutConflictCheck(ctx context.Context, activityID int64, roomID *int64) int64 {
	finalRoomID, _ := s.determineRoomIDWithStrategy(ctx, activityID, roomID, RoomConflictIgnore)
	return finalRoomID
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
		fmt.Printf("Warning: Overriding room conflict for room %d\n", roomID)
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
