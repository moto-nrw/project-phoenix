package iot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
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
			log.Printf("[engine] %s action failed: %v", actionCfg.Type, err)
		} else {
			e.metrics.recordSuccess(actionCfg.Type)
			executed[actionCfg.Type]++
		}
	}

	if len(executed) > 0 {
		parts := make([]string, 0, len(executed))
		for action, count := range executed {
			parts = append(parts, fmt.Sprintf("%s=%d", action, count))
		}
		log.Printf("[engine] tick summary: %s", strings.Join(parts, " "))
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

// Placeholder implementations; filled in with logic later.
func (e *Engine) executeCheckIn(ctx context.Context, action ActionConfig) error {
	type candidate struct {
		deviceID    string
		studentID   int64
		studentRFID string
		roomID      int64
		phase       RotationPhase
	}

	candidates := make([]candidate, 0)
	now := time.Now()

	e.stateMu.RLock()
	for deviceID, state := range e.states {
		if state == nil {
			continue
		}
		if !e.isDeviceAllowed(action, deviceID) {
			continue
		}
		if !state.sessionActive() || state.Session == nil || state.Session.RoomID == nil {
			continue
		}
		roomID := *state.Session.RoomID
		for _, student := range state.StudentStates {
			if student == nil {
				continue
			}
			if student.RFIDTag == "" {
				continue
			}
			if student.CurrentRoomID != nil {
				continue
			}
			if student.HasActiveVisit {
				continue
			}
			if !student.VisitCooldownUntil.IsZero() && student.VisitCooldownUntil.After(now) {
				continue
			}
			nextPhase := student.NextPhase
			if nextPhase == "" {
				nextPhase = RotationPhaseAG
			}
			if nextPhase == RotationPhaseSchulhof {
				continue
			}
			if nextPhase == RotationPhaseAG {
				if student.VisitedAGs != nil {
					if _, seen := student.VisitedAGs[roomID]; seen && len(student.VisitedAGs) < len(state.Activities) {
						continue
					}
				}
			}

			candidates = append(candidates, candidate{
				deviceID:    deviceID,
				studentID:   student.StudentID,
				studentRFID: student.RFIDTag,
				roomID:      roomID,
				phase:       nextPhase,
			})
		}
	}
	e.stateMu.RUnlock()

	if len(candidates) == 0 {
		return ErrNoEligibleCandidates
	}

	selected := candidates[e.randIntn(len(candidates))]

	deviceCfg, ok := e.deviceConfig(selected.deviceID)
	if !ok {
		return fmt.Errorf("device %s not configured", selected.deviceID)
	}

	resp, err := e.client.PerformCheckAction(ctx, deviceCfg, CheckActionPayload{
		StudentRFID: selected.studentRFID,
		Action:      "checkin",
		RoomID:      ptrInt64(selected.roomID),
	})
	if err != nil {
		e.stateMu.Lock()
		if state := e.states[selected.deviceID]; state != nil {
			if student := state.StudentStates[selected.studentID]; student != nil {
				ts := time.Now()
				student.LastEventAt = ts
				if strings.Contains(err.Error(), "student already has an active visit") {
					student.HasActiveVisit = true
				}
				student.VisitCooldownUntil = ts.Add(visitCooldown)
			}
		}
		e.stateMu.Unlock()
		return err
	}

	eventTime := time.Now()

	e.stateMu.Lock()
	defer e.stateMu.Unlock()

	state := e.states[selected.deviceID]
	if state == nil {
		return nil
	}
	student := state.StudentStates[selected.studentID]
	if student == nil {
		return nil
	}

	roomID := selected.roomID
	student.CurrentRoomID = ptrInt64(roomID)
	student.CurrentPhase = selected.phase
	student.LastEventAt = eventTime
	student.HasActiveVisit = true
	student.VisitCooldownUntil = eventTime.Add(visitCooldown)

	switch selected.phase {
	case RotationPhaseAG:
		if student.VisitedAGs == nil {
			student.VisitedAGs = make(map[int64]time.Time)
		}
		if _, seen := student.VisitedAGs[roomID]; !seen {
			student.AGHopCount++
		}
		student.VisitedAGs[roomID] = eventTime
		if student.AGHopTarget <= 0 {
			student.AGHopTarget = generateAGHopTarget(e.cfg.Event)
		}
		if student.AGHopCount >= student.AGHopTarget {
			student.NextPhase = RotationPhaseSchulhof
		} else {
			student.NextPhase = RotationPhaseAG
		}
	case RotationPhaseHeimatraum:
		student.AGHopCount = 0
		student.VisitedAGs = make(map[int64]time.Time)
		student.AGHopTarget = generateAGHopTarget(e.cfg.Event)
		student.NextPhase = RotationPhaseAG
		student.HomeRoomID = ptrInt64(roomID)
		student.HomeDeviceID = selected.deviceID
	case RotationPhaseSchulhof:
		student.NextPhase = RotationPhaseHeimatraum
	}

	log.Printf("[engine] checkin -> device=%s student=%d phase=%s visit_id=%v", selected.deviceID, selected.studentID, selected.phase, resp.VisitID)

	return nil
}

func (e *Engine) executeCheckOut(ctx context.Context, action ActionConfig) error {
	type candidate struct {
		deviceID    string
		studentID   int64
		studentRFID string
		phase       RotationPhase
	}

	candidates := make([]candidate, 0)
	now := time.Now()
	cutoff := now.Add(-e.cfg.Event.Interval / 2)

	e.stateMu.RLock()
	for deviceID, state := range e.states {
		if state == nil {
			continue
		}
		if !e.isDeviceAllowed(action, deviceID) {
			continue
		}
		if !state.sessionActive() {
			continue
		}
		for _, student := range state.StudentStates {
			if student == nil {
				continue
			}
			if student.RFIDTag == "" {
				continue
			}
			if student.CurrentRoomID == nil {
				continue
			}
			if !student.HasActiveVisit {
				continue
			}
			if !student.VisitCooldownUntil.IsZero() && student.VisitCooldownUntil.After(now) {
				continue
			}
			if !student.LastEventAt.IsZero() && student.LastEventAt.After(cutoff) {
				continue
			}

			candidates = append(candidates, candidate{
				deviceID:    deviceID,
				studentID:   student.StudentID,
				studentRFID: student.RFIDTag,
				phase:       student.CurrentPhase,
			})
		}
	}
	e.stateMu.RUnlock()

	if len(candidates) == 0 {
		return ErrNoEligibleCandidates
	}

	selected := candidates[e.randIntn(len(candidates))]

	deviceCfg, ok := e.deviceConfig(selected.deviceID)
	if !ok {
		return fmt.Errorf("device %s not configured", selected.deviceID)
	}

	_, err := e.client.PerformCheckAction(ctx, deviceCfg, CheckActionPayload{
		StudentRFID: selected.studentRFID,
		Action:      "checkout",
	})
	if err != nil {
		e.stateMu.Lock()
		if state := e.states[selected.deviceID]; state != nil {
			if student := state.StudentStates[selected.studentID]; student != nil {
				ts := time.Now()
				student.LastEventAt = ts
				student.VisitCooldownUntil = ts.Add(visitCooldown)
			}
		}
		e.stateMu.Unlock()
		return err
	}

	eventTime := time.Now()

	e.stateMu.Lock()
	defer e.stateMu.Unlock()

	state := e.states[selected.deviceID]
	if state == nil {
		return nil
	}
	student := state.StudentStates[selected.studentID]
	if student == nil {
		return nil
	}

	student.CurrentRoomID = nil
	student.LastEventAt = eventTime
	student.HasActiveVisit = false
	student.VisitCooldownUntil = eventTime.Add(visitCooldown)

	switch student.CurrentPhase {
	case RotationPhaseAG:
		if student.AGHopTarget <= 0 {
			student.AGHopTarget = generateAGHopTarget(e.cfg.Event)
		}
		if student.AGHopCount >= student.AGHopTarget {
			student.NextPhase = RotationPhaseSchulhof
		} else {
			student.NextPhase = RotationPhaseAG
		}
	case RotationPhaseSchulhof:
		student.NextPhase = RotationPhaseHeimatraum
		student.AGHopCount = 0
		student.VisitedAGs = make(map[int64]time.Time)
		student.AGHopTarget = generateAGHopTarget(e.cfg.Event)
	case RotationPhaseHeimatraum:
		student.NextPhase = RotationPhaseAG
	}

	log.Printf("[engine] checkout -> device=%s student=%d phase=%s", selected.deviceID, selected.studentID, student.CurrentPhase)

	return nil
}

func (e *Engine) executeSchulhofHop(ctx context.Context, action ActionConfig) error {
	type candidate struct {
		deviceID    string
		studentID   int64
		studentRFID string
		roomID      int64
		apiAction   string
	}

	candidates := make([]candidate, 0)
	now := time.Now()
	cutoff := now.Add(-e.cfg.Event.Interval / 2)

	e.stateMu.RLock()
	for deviceID, state := range e.states {
		if state == nil {
			continue
		}
		if !e.isDeviceAllowed(action, deviceID) {
			continue
		}
		if !state.sessionActive() || state.Session == nil || state.Session.RoomID == nil {
			continue
		}
		roomID := *state.Session.RoomID
		for _, student := range state.StudentStates {
			if student == nil {
				continue
			}
			if student.RFIDTag == "" {
				continue
			}
			if student.CurrentPhase == RotationPhaseSchulhof && student.CurrentRoomID != nil {
				if !student.HasActiveVisit {
					continue
				}
				if !student.VisitCooldownUntil.IsZero() && student.VisitCooldownUntil.After(now) {
					continue
				}
				if !student.LastEventAt.IsZero() && student.LastEventAt.After(cutoff) {
					continue
				}
				candidates = append(candidates, candidate{
					deviceID:    deviceID,
					studentID:   student.StudentID,
					studentRFID: student.RFIDTag,
					roomID:      roomID,
					apiAction:   "checkout",
				})
				continue
			}

			if student.NextPhase == RotationPhaseSchulhof && student.CurrentRoomID == nil {
				if student.HasActiveVisit {
					continue
				}
				if !student.VisitCooldownUntil.IsZero() && student.VisitCooldownUntil.After(now) {
					continue
				}
				candidates = append(candidates, candidate{
					deviceID:    deviceID,
					studentID:   student.StudentID,
					studentRFID: student.RFIDTag,
					roomID:      roomID,
					apiAction:   "checkin",
				})
			}
		}
	}
	e.stateMu.RUnlock()

	if len(candidates) == 0 {
		return ErrNoEligibleCandidates
	}

	selected := candidates[e.randIntn(len(candidates))]

	deviceCfg, ok := e.deviceConfig(selected.deviceID)
	if !ok {
		return fmt.Errorf("device %s not configured", selected.deviceID)
	}

	payload := CheckActionPayload{
		StudentRFID: selected.studentRFID,
		Action:      selected.apiAction,
	}
	if selected.apiAction == "checkin" {
		payload.RoomID = ptrInt64(selected.roomID)
	}

	_, err := e.client.PerformCheckAction(ctx, deviceCfg, payload)
	if err != nil {
		e.stateMu.Lock()
		if state := e.states[selected.deviceID]; state != nil {
			if student := state.StudentStates[selected.studentID]; student != nil {
				ts := time.Now()
				student.LastEventAt = ts
				student.VisitCooldownUntil = ts.Add(visitCooldown)
			}
		}
		e.stateMu.Unlock()
		return err
	}

	eventTime := time.Now()

	e.stateMu.Lock()
	defer e.stateMu.Unlock()

	state := e.states[selected.deviceID]
	if state == nil {
		return nil
	}
	student := state.StudentStates[selected.studentID]
	if student == nil {
		return nil
	}

	if selected.apiAction == "checkin" {
		student.CurrentRoomID = ptrInt64(selected.roomID)
		student.CurrentPhase = RotationPhaseSchulhof
		student.NextPhase = RotationPhaseHeimatraum
		student.LastEventAt = eventTime
		student.HasActiveVisit = true
		student.VisitCooldownUntil = eventTime.Add(visitCooldown)
	} else {
		student.CurrentRoomID = nil
		student.CurrentPhase = RotationPhaseSchulhof
		student.NextPhase = RotationPhaseHeimatraum
		student.LastEventAt = eventTime
		student.AGHopCount = 0
		student.VisitedAGs = make(map[int64]time.Time)
		student.AGHopTarget = generateAGHopTarget(e.cfg.Event)
		student.HasActiveVisit = false
		student.VisitCooldownUntil = eventTime.Add(visitCooldown)
	}

	log.Printf("[engine] schulhof_%s -> device=%s student=%d", selected.apiAction, selected.deviceID, selected.studentID)

	return nil
}

func (e *Engine) executeAttendanceToggle(ctx context.Context, action ActionConfig) error {
	type candidate struct {
		deviceID    string
		studentID   int64
		studentRFID string
	}

	candidates := make([]candidate, 0)
	cutoff := time.Now().Add(-e.cfg.Event.Interval)

	e.stateMu.RLock()
	for deviceID, state := range e.states {
		if state == nil {
			continue
		}
		if !e.isDeviceAllowed(action, deviceID) {
			continue
		}
		if !state.sessionActive() || state.Session == nil || state.Session.RoomID == nil {
			continue
		}
		roomID := *state.Session.RoomID
		leadPresent := false
		for _, sup := range state.ActiveSupervisors {
			if sup.IsLead {
				leadPresent = true
				break
			}
		}
		if !leadPresent {
			continue
		}
		for _, student := range state.StudentStates {
			if student == nil {
				continue
			}
			if student.RFIDTag == "" {
				continue
			}
			if student.HomeRoomID == nil || student.CurrentRoomID == nil {
				continue
			}
			if student.CurrentPhase != RotationPhaseHeimatraum {
				continue
			}
			if *student.HomeRoomID != roomID {
				continue
			}
			if student.HomeDeviceID != "" && student.HomeDeviceID != deviceID {
				continue
			}
			if !student.LastAttendance.IsZero() && student.LastAttendance.After(cutoff) {
				continue
			}
			candidates = append(candidates, candidate{
				deviceID:    deviceID,
				studentID:   student.StudentID,
				studentRFID: student.RFIDTag,
			})
		}
	}
	e.stateMu.RUnlock()

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
		e.stateMu.Lock()
		if state := e.states[selected.deviceID]; state != nil {
			if student := state.StudentStates[selected.studentID]; student != nil {
				student.LastAttendance = time.Now()
				student.LastEventAt = student.LastAttendance
			}
		}
		e.stateMu.Unlock()
		return err
	}

	now := time.Now()

	e.stateMu.Lock()
	defer e.stateMu.Unlock()

	state := e.states[selected.deviceID]
	if state == nil {
		return nil
	}
	student := state.StudentStates[selected.studentID]
	if student == nil {
		return nil
	}

	student.AttendanceStatus = resp.Attendance.Status
	student.LastAttendance = now
	student.LastEventAt = now

	log.Printf("[engine] attendance_toggle -> device=%s student=%d status=%s", selected.deviceID, selected.studentID, student.AttendanceStatus)

	return nil
}

func (e *Engine) executeSupervisorSwap(ctx context.Context, action ActionConfig) error {
	type swapCandidate struct {
		deviceID        string
		sessionID       int64
		replaceStaffID  int64
		replacementID   int64
		currentIDs      []int64
		replacementLead bool
	}

	candidates := make([]swapCandidate, 0)

	e.stateMu.RLock()
	for deviceID, state := range e.states {
		if state == nil {
			continue
		}
		if !e.isDeviceAllowed(action, deviceID) {
			continue
		}
		if !state.sessionActive() {
			continue
		}
		var sessionID int64
		switch {
		case state.Session != nil && state.Session.ActiveGroupID != nil:
			sessionID = *state.Session.ActiveGroupID
		case state.ManagedSessionID != nil:
			sessionID = *state.ManagedSessionID
		default:
			continue
		}

		deviceCfg, ok := e.deviceConfig(deviceID)
		if !ok {
			continue
		}

		assigned := make(map[int64]SupervisorAssignment)
		for id, slot := range state.ActiveSupervisors {
			assigned[id] = slot
		}

		if len(assigned) == 0 && deviceCfg.DefaultSession != nil {
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

		if len(assigned) == 0 {
			// Seed with any available staff to avoid empty supervisor list
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

		if len(assigned) == 0 {
			continue
		}

		nonLeadAssigned := make([]SupervisorAssignment, 0)
		currentOrder := make([]int64, 0, len(assigned))
		for id, slot := range assigned {
			currentOrder = append(currentOrder, id)
			staff := state.StaffRoster[id]
			if staff != nil && !staff.IsLead {
				nonLeadAssigned = append(nonLeadAssigned, slot)
			}
		}

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

		if len(available) == 0 || len(nonLeadAssigned) == 0 {
			continue
		}

		replace := nonLeadAssigned[e.randIntn(len(nonLeadAssigned))]
		replacement := available[e.randIntn(len(available))]

		nextIDs := make([]int64, 0, len(assigned))
		for _, id := range currentOrder {
			if id == replace.StaffID {
				continue
			}
			nextIDs = append(nextIDs, id)
		}
		nextIDs = append(nextIDs, replacement.StaffID)

		// Ensure at least one lead remains
		leadPresent := false
		for _, id := range nextIDs {
			if staff := state.StaffRoster[id]; staff != nil && staff.IsLead {
				leadPresent = true
				break
			}
		}
		if !leadPresent {
			continue
		}

		candidates = append(candidates, swapCandidate{
			deviceID:        deviceID,
			sessionID:       sessionID,
			replaceStaffID:  replace.StaffID,
			replacementID:   replacement.StaffID,
			currentIDs:      nextIDs,
			replacementLead: replacement.IsLead,
		})
	}
	e.stateMu.RUnlock()

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

	now := time.Now()

	e.stateMu.Lock()
	defer e.stateMu.Unlock()

	state := e.states[selected.deviceID]
	if state == nil {
		return nil
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

	log.Printf("[engine] supervisor_swap -> device=%s session=%d out=%d in=%d", selected.deviceID, selected.sessionID, selected.replaceStaffID, selected.replacementID)

	return nil
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
