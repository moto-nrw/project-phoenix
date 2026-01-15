package iot

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/api/iot/attendance"
	"github.com/moto-nrw/project-phoenix/logging"
)

var (
	// ErrNoEligibleCandidates indicates that an action had no applicable devices or students.
	ErrNoEligibleCandidates = errors.New("no eligible candidates for action")
)

const visitCooldown = 3 * time.Second

// Engine drives simulated events against the API based on cached discovery state.
type Engine struct {
	cfg     *Config
	client  *Client
	stateMu *sync.RWMutex
	states  map[string]*DeviceState

	metrics *EngineMetrics

	randMu sync.Mutex
	rand   *rand.Rand

	deviceConfigs map[string]DeviceConfig
}

// EngineMetrics tracks how many actions were executed.
type EngineMetrics struct {
	mu       sync.Mutex
	counts   map[ActionType]int64
	failures map[ActionType]int64
}

// NewEngine creates a new event engine instance.
func NewEngine(cfg *Config, client *Client, stateMu *sync.RWMutex, states map[string]*DeviceState) *Engine {
	configs := make(map[string]DeviceConfig, len(cfg.Devices))
	for _, device := range cfg.Devices {
		configs[device.DeviceID] = device
	}

	return &Engine{
		cfg:     cfg,
		client:  client,
		stateMu: stateMu,
		states:  states,
		metrics: &EngineMetrics{
			counts:   make(map[ActionType]int64),
			failures: make(map[ActionType]int64),
		},
		rand:          rand.New(rand.NewSource(time.Now().UnixNano())),
		deviceConfigs: configs,
	}
}

// Tick executes up to max_events_per_tick actions.
func (e *Engine) Tick(ctx context.Context) {
	maxEvents := e.cfg.Event.MaxEventsPerTick
	if maxEvents <= 0 {
		return
	}

	executed := make(map[ActionType]int)

	for i := 0; i < maxEvents; i++ {
		actionCfg, ok := e.selectAction()
		if !ok {
			return
		}

		if err := e.executeAction(ctx, actionCfg); err != nil {
			if errors.Is(err, ErrNoEligibleCandidates) {
				continue
			}
			e.metrics.recordFailure(actionCfg.Type)
			if logging.Logger != nil {
				logging.Logger.WithFields(map[string]interface{}{
					"action_type": string(actionCfg.Type),
					"error":       err.Error(),
				}).Warn("Action failed")
			}
		} else {
			e.metrics.recordSuccess(actionCfg.Type)
			executed[actionCfg.Type]++
		}
	}

	if len(executed) > 0 && logging.Logger != nil {
		parts := make([]string, 0, len(executed))
		for action, count := range executed {
			parts = append(parts, fmt.Sprintf("%s=%d", action, count))
		}
		logging.Logger.WithField("summary", strings.Join(parts, " ")).Debug("Tick summary")
	}
}

func (e *Engine) selectAction() (ActionConfig, bool) {
	candidates := make([]ActionConfig, 0, len(e.cfg.Event.Actions))
	var totalWeight float64
	for _, action := range e.cfg.Event.Actions {
		if action.Disabled {
			continue
		}
		if action.Weight <= 0 {
			continue
		}
		candidates = append(candidates, action)
		totalWeight += action.Weight
	}

	if len(candidates) == 0 || totalWeight <= 0 {
		return ActionConfig{}, false
	}

	e.randMu.Lock()
	r := e.rand.Float64() * totalWeight
	e.randMu.Unlock()

	var cumulative float64
	for _, action := range candidates {
		cumulative += action.Weight
		if r < cumulative {
			return action, true
		}
	}

	// Fallback to last candidate (should not typically happen due to floating point rounding)
	return candidates[len(candidates)-1], true
}

func (e *Engine) executeAction(ctx context.Context, action ActionConfig) error {
	switch action.Type {
	case ActionCheckIn:
		return e.executeCheckIn(ctx, action)
	case ActionCheckOut:
		return e.executeCheckOut(ctx, action)
	case ActionSchulhofHop:
		return e.executeSchulhofHop(ctx, action)
	case ActionAttendanceToggle:
		return e.executeAttendanceToggle(ctx, action)
	case ActionSupervisorSwap:
		return e.executeSupervisorSwap(ctx, action)
	default:
		return fmt.Errorf("unsupported action type %q", action.Type)
	}
}

func (m *EngineMetrics) recordSuccess(action ActionType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[action]++
}

func (m *EngineMetrics) recordFailure(action ActionType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures[action]++
}

// errDeviceNotConfigured returns an error for unconfigured devices.
func errDeviceNotConfigured(deviceID string) error {
	return fmt.Errorf("device %s not configured", deviceID)
}

// attendanceCandidate represents a student eligible for attendance toggle.
type attendanceCandidate struct {
	deviceID    string
	studentID   int64
	studentRFID string
}

// executeAttendanceToggle performs an attendance toggle for a randomly selected eligible student.
func (e *Engine) executeAttendanceToggle(ctx context.Context, action ActionConfig) error {
	candidates := e.collectAttendanceCandidates(action)
	if len(candidates) == 0 {
		return ErrNoEligibleCandidates
	}

	selected := candidates[e.randIntn(len(candidates))]
	deviceCfg, ok := e.deviceConfig(selected.deviceID)
	if !ok {
		return fmt.Errorf("device %s not configured", selected.deviceID)
	}

	resp, err := e.client.ToggleAttendance(ctx, deviceCfg, AttendanceTogglePayload{
		RFID:   selected.studentRFID,
		Action: "confirm",
	})
	if err != nil {
		e.handleAttendanceError(selected)
		return err
	}

	e.updateStateAfterAttendance(selected, resp)
	return nil
}

// collectAttendanceCandidates finds all students eligible for attendance toggle.
func (e *Engine) collectAttendanceCandidates(action ActionConfig) []attendanceCandidate {
	candidates := make([]attendanceCandidate, 0)
	cutoff := time.Now().Add(-e.cfg.Event.Interval)

	e.stateMu.RLock()
	defer e.stateMu.RUnlock()

	for deviceID, state := range e.states {
		if !e.isValidAttendanceDevice(action, deviceID, state) {
			continue
		}
		roomID := *state.Session.RoomID
		e.collectAttendanceStudents(&candidates, deviceID, state, roomID, cutoff)
	}
	return candidates
}

// isValidAttendanceDevice checks if a device is valid for attendance actions.
func (e *Engine) isValidAttendanceDevice(action ActionConfig, deviceID string, state *DeviceState) bool {
	if !e.isValidCheckInDevice(action, deviceID, state) {
		return false
	}
	return e.hasLeadSupervisor(state)
}

// hasLeadSupervisor checks if the device state has a lead supervisor.
func (e *Engine) hasLeadSupervisor(state *DeviceState) bool {
	for _, sup := range state.ActiveSupervisors {
		if sup.IsLead {
			return true
		}
	}
	return false
}

// collectAttendanceStudents adds eligible students from a device to the attendance candidates list.
func (e *Engine) collectAttendanceStudents(candidates *[]attendanceCandidate, deviceID string, state *DeviceState, roomID int64, cutoff time.Time) {
	for _, student := range state.StudentStates {
		if !e.isEligibleForAttendance(student, deviceID, roomID, cutoff) {
			continue
		}
		*candidates = append(*candidates, attendanceCandidate{
			deviceID:    deviceID,
			studentID:   student.StudentID,
			studentRFID: student.RFIDTag,
		})
	}
}

// isEligibleForAttendance checks if a student is eligible for attendance toggle.
func (e *Engine) isEligibleForAttendance(student *StudentState, deviceID string, roomID int64, cutoff time.Time) bool {
	if student == nil || student.RFIDTag == "" {
		return false
	}
	if student.HomeRoomID == nil || student.CurrentRoomID == nil {
		return false
	}
	if student.CurrentPhase != RotationPhaseHeimatraum {
		return false
	}
	if *student.HomeRoomID != roomID {
		return false
	}
	if student.HomeDeviceID != "" && student.HomeDeviceID != deviceID {
		return false
	}
	if !student.LastAttendance.IsZero() && student.LastAttendance.After(cutoff) {
		return false
	}
	return true
}

// handleAttendanceError updates student state when attendance toggle fails.
func (e *Engine) handleAttendanceError(selected attendanceCandidate) {
	e.stateMu.Lock()
	defer e.stateMu.Unlock()

	state := e.states[selected.deviceID]
	if state == nil {
		return
	}
	student := state.StudentStates[selected.studentID]
	if student == nil {
		return
	}
	student.LastAttendance = time.Now()
	student.LastEventAt = student.LastAttendance
}

// updateStateAfterAttendance updates student state after successful attendance toggle.
func (e *Engine) updateStateAfterAttendance(selected attendanceCandidate, resp *attendance.AttendanceToggleResponse) {
	now := time.Now()

	e.stateMu.Lock()
	defer e.stateMu.Unlock()

	state := e.states[selected.deviceID]
	if state == nil {
		return
	}
	student := state.StudentStates[selected.studentID]
	if student == nil {
		return
	}

	student.AttendanceStatus = resp.Attendance.Status
	student.LastAttendance = now
	student.LastEventAt = now

	if logging.Logger != nil {
		logging.Logger.WithFields(map[string]interface{}{
			"device_id":  selected.deviceID,
			"student_id": selected.studentID,
			"status":     student.AttendanceStatus,
		}).Debug("Attendance toggle completed")
	}
}

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

	if logging.Logger != nil {
		logging.Logger.WithFields(map[string]interface{}{
			"device_id":   selected.deviceID,
			"session_id":  selected.sessionID,
			"out_staff":   selected.replaceStaffID,
			"in_staff":    selected.replacementID,
		}).Debug("Supervisor swap completed")
	}
}

func (e *Engine) isDeviceAllowed(action ActionConfig, deviceID string) bool {
	if len(action.DeviceIDs) == 0 {
		return true
	}
	for _, allowed := range action.DeviceIDs {
		if allowed == deviceID {
			return true
		}
	}
	return false
}

func (e *Engine) deviceConfig(deviceID string) (DeviceConfig, bool) {
	cfg, ok := e.deviceConfigs[deviceID]
	return cfg, ok
}

func (e *Engine) randIntn(n int) int {
	e.randMu.Lock()
	defer e.randMu.Unlock()
	return e.rand.Intn(n)
}

func ptrInt64(v int64) *int64 {
	vv := v
	return &vv
}

func isVisitMissingError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "visit not found") ||
		strings.Contains(msg, "no active visit") ||
		strings.Contains(msg, "room_id is required for check-in")
}
