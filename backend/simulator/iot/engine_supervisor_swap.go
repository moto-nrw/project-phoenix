package iot

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
)

// swapCandidate represents a supervisor swap action candidate.
type swapCandidate struct {
	deviceID        string
	sessionID       int64
	replaceStaffID  int64
	replacementID   int64
	currentIDs      []int64
	replacementLead bool
}

// executeSupervisorSwap performs a supervisor swap for a randomly selected eligible device.
func (e *Engine) executeSupervisorSwap(ctx context.Context, action ActionConfig) error {
	candidates := e.collectSupervisorSwapCandidates(action)
	if len(candidates) == 0 {
		return ErrNoEligibleCandidates
	}

	selected := candidates[e.randIntn(len(candidates))]
	deviceCfg, ok := e.deviceConfig(selected.deviceID)
	if !ok {
		return fmt.Errorf("device %s not configured", selected.deviceID)
	}

	_, err := e.client.UpdateSessionSupervisors(ctx, deviceCfg, selected.sessionID, selected.currentIDs)
	if err != nil {
		return err
	}

	e.updateStateAfterSupervisorSwap(selected)
	return nil
}

// collectSupervisorSwapCandidates finds all devices eligible for supervisor swap.
func (e *Engine) collectSupervisorSwapCandidates(action ActionConfig) []swapCandidate {
	candidates := make([]swapCandidate, 0)

	e.stateMu.RLock()
	defer e.stateMu.RUnlock()

	for deviceID, state := range e.states {
		candidate := e.buildSwapCandidate(action, deviceID, state)
		if candidate != nil {
			candidates = append(candidates, *candidate)
		}
	}
	return candidates
}

// buildSwapCandidate builds a swap candidate for a single device if eligible.
func (e *Engine) buildSwapCandidate(action ActionConfig, deviceID string, state *DeviceState) *swapCandidate {
	if !e.isValidSwapDevice(action, deviceID, state) {
		return nil
	}

	sessionID := e.getSessionID(state)
	if sessionID == 0 {
		return nil
	}

	deviceCfg, ok := e.deviceConfig(deviceID)
	if !ok {
		return nil
	}

	assigned := e.buildAssignedSupervisors(state, deviceCfg)
	if len(assigned) == 0 {
		return nil
	}

	nonLeadAssigned, currentOrder := e.categorizeAssigned(state, assigned)
	available := e.findAvailableStaff(state, assigned)

	if len(available) == 0 || len(nonLeadAssigned) == 0 {
		return nil
	}

	replace := nonLeadAssigned[e.randIntn(len(nonLeadAssigned))]
	replacement := available[e.randIntn(len(available))]
	nextIDs := e.buildNextIDs(currentOrder, replace.StaffID, replacement.StaffID)

	if !e.hasLeadInIDs(state, nextIDs) {
		return nil
	}

	return &swapCandidate{
		deviceID:        deviceID,
		sessionID:       sessionID,
		replaceStaffID:  replace.StaffID,
		replacementID:   replacement.StaffID,
		currentIDs:      nextIDs,
		replacementLead: replacement.IsLead,
	}
}

// isValidSwapDevice checks if a device is valid for supervisor swap.
// Swap requires the same conditions as check-out: valid device with active session.
func (e *Engine) isValidSwapDevice(action ActionConfig, deviceID string, state *DeviceState) bool {
	return e.isValidCheckOutDevice(action, deviceID, state)
}

// getSessionID extracts the session ID from state.
func (e *Engine) getSessionID(state *DeviceState) int64 {
	if state.Session != nil && state.Session.ActiveGroupID != nil {
		return *state.Session.ActiveGroupID
	}
	if state.ManagedSessionID != nil {
		return *state.ManagedSessionID
	}
	return 0
}

// buildAssignedSupervisors builds the map of currently assigned supervisors.
func (e *Engine) buildAssignedSupervisors(state *DeviceState, deviceCfg DeviceConfig) map[int64]SupervisorAssignment {
	assigned := make(map[int64]SupervisorAssignment)

	// Copy current assignments
	for id, slot := range state.ActiveSupervisors {
		assigned[id] = slot
	}

	// Use default session supervisors if none assigned
	if len(assigned) == 0 && deviceCfg.DefaultSession != nil {
		e.addDefaultSupervisors(state, assigned, deviceCfg)
	}

	// Seed with any available staff if still empty
	if len(assigned) == 0 {
		e.seedWithAvailableStaff(state, assigned)
	}

	return assigned
}

// addDefaultSupervisors adds supervisors from default session config.
func (e *Engine) addDefaultSupervisors(state *DeviceState, assigned map[int64]SupervisorAssignment, deviceCfg DeviceConfig) {
	for _, supID := range deviceCfg.DefaultSession.SupervisorIDs {
		if supID <= 0 {
			continue
		}
		staff, ok := state.StaffRoster[supID]
		if !ok {
			continue
		}
		assigned[supID] = SupervisorAssignment{
			StaffID:     supID,
			IsLead:      staff.IsLead,
			LastUpdated: time.Now().Add(-2 * e.cfg.Event.Interval),
		}
	}
}

// seedWithAvailableStaff seeds assigned map with any available staff.
func (e *Engine) seedWithAvailableStaff(state *DeviceState, assigned map[int64]SupervisorAssignment) {
	for _, staff := range state.StaffRoster {
		assigned[staff.StaffID] = SupervisorAssignment{
			StaffID:     staff.StaffID,
			IsLead:      staff.IsLead,
			LastUpdated: time.Now().Add(-2 * e.cfg.Event.Interval),
		}
		if len(assigned) >= 2 {
			break
		}
	}
}

// categorizeAssigned categorizes assigned supervisors into leads and non-leads.
func (e *Engine) categorizeAssigned(state *DeviceState, assigned map[int64]SupervisorAssignment) ([]SupervisorAssignment, []int64) {
	nonLeadAssigned := make([]SupervisorAssignment, 0)
	currentOrder := make([]int64, 0, len(assigned))

	for id, slot := range assigned {
		currentOrder = append(currentOrder, id)
		staff := state.StaffRoster[id]
		if staff != nil && !staff.IsLead {
			nonLeadAssigned = append(nonLeadAssigned, slot)
		}
	}
	return nonLeadAssigned, currentOrder
}

// findAvailableStaff finds staff not currently assigned.
func (e *Engine) findAvailableStaff(state *DeviceState, assigned map[int64]SupervisorAssignment) []*StaffState {
	available := make([]*StaffState, 0)
	for id, staff := range state.StaffRoster {
		if staff == nil {
			continue
		}
		if _, already := assigned[id]; already {
			continue
		}
		available = append(available, staff)
	}
	return available
}

// buildNextIDs builds the new supervisor ID list after swap.
func (e *Engine) buildNextIDs(currentOrder []int64, replaceID, replacementID int64) []int64 {
	nextIDs := make([]int64, 0, len(currentOrder))
	for _, id := range currentOrder {
		if id == replaceID {
			continue
		}
		nextIDs = append(nextIDs, id)
	}
	nextIDs = append(nextIDs, replacementID)
	return nextIDs
}

// hasLeadInIDs checks if at least one lead is present in the IDs.
func (e *Engine) hasLeadInIDs(state *DeviceState, ids []int64) bool {
	for _, id := range ids {
		if staff := state.StaffRoster[id]; staff != nil && staff.IsLead {
			return true
		}
	}
	return false
}

// updateStateAfterSupervisorSwap updates state after successful supervisor swap.
func (e *Engine) updateStateAfterSupervisorSwap(selected swapCandidate) {
	now := time.Now()

	e.stateMu.Lock()
	defer e.stateMu.Unlock()

	state := e.states[selected.deviceID]
	if state == nil {
		return
	}

	// Reset supervisor assignments to reflect the latest update
	state.ActiveSupervisors = make(map[int64]SupervisorAssignment, len(selected.currentIDs))
	for _, id := range selected.currentIDs {
		staff := state.StaffRoster[id]
		state.ActiveSupervisors[id] = SupervisorAssignment{
			StaffID:     id,
			IsLead:      staff != nil && staff.IsLead,
			LastUpdated: now,
		}
		if staff != nil {
			staff.LastActive = now
		}
	}

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"device_id":  selected.deviceID,
			"session_id": selected.sessionID,
			"out_staff":  selected.replaceStaffID,
			"in_staff":   selected.replacementID,
		}).Debug("Supervisor swap completed")
	}
}
