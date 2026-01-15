package iot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
)

// checkInCandidate represents a student eligible for check-in.
type checkInCandidate struct {
	deviceID    string
	studentID   int64
	studentRFID string
	roomID      int64
	phase       RotationPhase
}

// executeCheckIn performs a check-in action for a randomly selected eligible student.
func (e *Engine) executeCheckIn(ctx context.Context, action ActionConfig) error {
	candidates := e.collectCheckInCandidates(action)
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
		e.handleCheckInError(selected, err)
		return err
	}

	e.updateStateAfterCheckIn(selected, resp)
	return nil
}

// collectCheckInCandidates finds all students eligible for check-in.
func (e *Engine) collectCheckInCandidates(action ActionConfig) []checkInCandidate {
	candidates := make([]checkInCandidate, 0)
	now := time.Now()

	e.stateMu.RLock()
	defer e.stateMu.RUnlock()

	for deviceID, state := range e.states {
		if !e.isValidCheckInDevice(action, deviceID, state) {
			continue
		}
		roomID := *state.Session.RoomID
		e.collectCheckInStudents(&candidates, deviceID, state, roomID, now)
	}
	return candidates
}

// isValidCheckInDevice checks if a device is valid for check-in actions.
func (e *Engine) isValidCheckInDevice(action ActionConfig, deviceID string, state *DeviceState) bool {
	if state == nil {
		return false
	}
	if !e.isDeviceAllowed(action, deviceID) {
		return false
	}
	return state.sessionActive() && state.Session != nil && state.Session.RoomID != nil
}

// collectCheckInStudents adds eligible students from a device to the candidates list.
func (e *Engine) collectCheckInStudents(candidates *[]checkInCandidate, deviceID string, state *DeviceState, roomID int64, now time.Time) {
	for _, student := range state.StudentStates {
		if !e.isEligibleForCheckIn(student, state, roomID, now) {
			continue
		}
		nextPhase := student.NextPhase
		if nextPhase == "" {
			nextPhase = RotationPhaseAG
		}
		*candidates = append(*candidates, checkInCandidate{
			deviceID:    deviceID,
			studentID:   student.StudentID,
			studentRFID: student.RFIDTag,
			roomID:      roomID,
			phase:       nextPhase,
		})
	}
}

// isEligibleForCheckIn checks if a student is eligible for check-in.
func (e *Engine) isEligibleForCheckIn(student *StudentState, state *DeviceState, roomID int64, now time.Time) bool {
	if student == nil || student.RFIDTag == "" {
		return false
	}
	if student.CurrentRoomID != nil || student.HasActiveVisit {
		return false
	}
	if !student.VisitCooldownUntil.IsZero() && student.VisitCooldownUntil.After(now) {
		return false
	}
	nextPhase := student.NextPhase
	if nextPhase == "" {
		nextPhase = RotationPhaseAG
	}
	if nextPhase == RotationPhaseSchulhof {
		return false
	}
	if nextPhase == RotationPhaseAG && student.VisitedAGs != nil {
		if _, seen := student.VisitedAGs[roomID]; seen && len(student.VisitedAGs) < len(state.Activities) {
			return false
		}
	}
	return true
}

// handleCheckInError updates student state when check-in fails.
func (e *Engine) handleCheckInError(selected checkInCandidate, err error) {
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
	ts := time.Now()
	student.LastEventAt = ts
	if strings.Contains(err.Error(), "student already has an active visit") {
		student.HasActiveVisit = true
	}
	student.VisitCooldownUntil = ts.Add(visitCooldown)
}

// updateStateAfterCheckIn updates student state after successful check-in.
func (e *Engine) updateStateAfterCheckIn(selected checkInCandidate, resp *checkin.CheckinResponse) {
	eventTime := time.Now()

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

	student.CurrentRoomID = ptrInt64(selected.roomID)
	student.CurrentPhase = selected.phase
	student.LastEventAt = eventTime
	student.HasActiveVisit = true
	student.VisitCooldownUntil = eventTime.Add(visitCooldown)

	e.updatePhaseAfterCheckIn(student, selected.phase, selected.roomID, selected.deviceID, eventTime)

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"device_id":  selected.deviceID,
			"student_id": selected.studentID,
			"phase":      string(selected.phase),
			"visit_id":   resp.VisitID,
		}).Debug("Check-in completed")
	}
}

// updatePhaseAfterCheckIn updates student phase-specific state after check-in.
func (e *Engine) updatePhaseAfterCheckIn(student *StudentState, phase RotationPhase, roomID int64, deviceID string, eventTime time.Time) {
	switch phase {
	case RotationPhaseAG:
		e.updateAGPhaseAfterCheckIn(student, roomID, eventTime)
	case RotationPhaseHeimatraum:
		e.updateHeimatraumPhaseAfterCheckIn(student, roomID, deviceID, eventTime)
	case RotationPhaseSchulhof:
		student.NextPhase = RotationPhaseHeimatraum
	}
}

// updateAGPhaseAfterCheckIn handles AG phase updates after check-in.
func (e *Engine) updateAGPhaseAfterCheckIn(student *StudentState, roomID int64, eventTime time.Time) {
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
}

// updateHeimatraumPhaseAfterCheckIn handles Heimatraum phase updates after check-in.
func (e *Engine) updateHeimatraumPhaseAfterCheckIn(student *StudentState, roomID int64, deviceID string, _ time.Time) {
	student.AGHopCount = 0
	student.VisitedAGs = make(map[int64]time.Time)
	student.AGHopTarget = generateAGHopTarget(e.cfg.Event)
	student.NextPhase = RotationPhaseAG
	student.HomeRoomID = ptrInt64(roomID)
	student.HomeDeviceID = deviceID
}
