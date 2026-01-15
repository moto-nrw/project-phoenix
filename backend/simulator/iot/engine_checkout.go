package iot

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/logging"
)

// checkOutCandidate represents a student eligible for check-out.
type checkOutCandidate struct {
	deviceID    string
	studentID   int64
	studentRFID string
	phase       RotationPhase
}

// executeCheckOut performs a check-out action for a randomly selected eligible student.
func (e *Engine) executeCheckOut(ctx context.Context, action ActionConfig) error {
	candidates := e.collectCheckOutCandidates(action)
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
		return e.handleCheckOutError(selected, err)
	}

	e.updateStateAfterCheckOut(selected)
	return nil
}

// collectCheckOutCandidates finds all students eligible for check-out.
func (e *Engine) collectCheckOutCandidates(action ActionConfig) []checkOutCandidate {
	candidates := make([]checkOutCandidate, 0)
	now := time.Now()
	cutoff := now.Add(-e.cfg.Event.Interval / 2)

	e.stateMu.RLock()
	defer e.stateMu.RUnlock()

	for deviceID, state := range e.states {
		if !e.isValidCheckOutDevice(action, deviceID, state) {
			continue
		}
		e.collectCheckOutStudents(&candidates, deviceID, state, now, cutoff)
	}
	return candidates
}

// isValidCheckOutDevice checks if a device is valid for check-out actions.
func (e *Engine) isValidCheckOutDevice(action ActionConfig, deviceID string, state *DeviceState) bool {
	if state == nil {
		return false
	}
	if !e.isDeviceAllowed(action, deviceID) {
		return false
	}
	return state.sessionActive()
}

// collectCheckOutStudents adds eligible students from a device to the candidates list.
func (e *Engine) collectCheckOutStudents(candidates *[]checkOutCandidate, deviceID string, state *DeviceState, now, cutoff time.Time) {
	for _, student := range state.StudentStates {
		if !e.isEligibleForCheckOut(student, now, cutoff) {
			continue
		}
		*candidates = append(*candidates, checkOutCandidate{
			deviceID:    deviceID,
			studentID:   student.StudentID,
			studentRFID: student.RFIDTag,
			phase:       student.CurrentPhase,
		})
	}
}

// isEligibleForCheckOut checks if a student is eligible for check-out.
func (e *Engine) isEligibleForCheckOut(student *StudentState, now, cutoff time.Time) bool {
	if student == nil || student.RFIDTag == "" {
		return false
	}
	if student.CurrentRoomID == nil || !student.HasActiveVisit {
		return false
	}
	if !student.VisitCooldownUntil.IsZero() && student.VisitCooldownUntil.After(now) {
		return false
	}
	if !student.LastEventAt.IsZero() && student.LastEventAt.After(cutoff) {
		return false
	}
	return true
}

// handleCheckOutError updates student state when check-out fails and returns appropriate error.
func (e *Engine) handleCheckOutError(selected checkOutCandidate, err error) error {
	missingVisit := isVisitMissingError(err)

	e.stateMu.Lock()
	if state := e.states[selected.deviceID]; state != nil {
		if student := state.StudentStates[selected.studentID]; student != nil {
			ts := time.Now()
			student.LastEventAt = ts
			student.VisitCooldownUntil = ts.Add(visitCooldown)
			if missingVisit {
				student.HasActiveVisit = false
				student.CurrentRoomID = nil
			}
		}
	}
	e.stateMu.Unlock()

	if missingVisit {
		return nil
	}
	return err
}

// updateStateAfterCheckOut updates student state after successful check-out.
func (e *Engine) updateStateAfterCheckOut(selected checkOutCandidate) {
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

	student.CurrentRoomID = nil
	student.LastEventAt = eventTime
	student.HasActiveVisit = false
	student.VisitCooldownUntil = eventTime.Add(visitCooldown)

	e.updatePhaseAfterCheckOut(student)

	if logging.Logger != nil {
		logging.Logger.WithFields(map[string]interface{}{
			"device_id":  selected.deviceID,
			"student_id": selected.studentID,
			"phase":      string(student.CurrentPhase),
		}).Debug("Check-out completed")
	}
}

// updatePhaseAfterCheckOut updates student phase-specific state after check-out.
func (e *Engine) updatePhaseAfterCheckOut(student *StudentState) {
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
}
