package iot

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/logging"
)

// schulhofCandidate represents a student eligible for Schulhof hop action.
type schulhofCandidate struct {
	deviceID    string
	studentID   int64
	studentRFID string
	roomID      int64
	apiAction   string
}

// executeSchulhofHop performs a Schulhof check-in or check-out for a randomly selected eligible student.
func (e *Engine) executeSchulhofHop(ctx context.Context, action ActionConfig) error {
	candidates := e.collectSchulhofCandidates(action)
	if len(candidates) == 0 {
		return ErrNoEligibleCandidates
	}

	selected := candidates[e.randIntn(len(candidates))]
	deviceCfg, ok := e.deviceConfig(selected.deviceID)
	if !ok {
		return errDeviceNotConfigured(selected.deviceID)
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
		return e.handleSchulhofError(selected, payload, err)
	}

	e.updateStateAfterSchulhofHop(selected)
	return nil
}

// collectSchulhofCandidates finds all students eligible for Schulhof hop actions.
func (e *Engine) collectSchulhofCandidates(action ActionConfig) []schulhofCandidate {
	candidates := make([]schulhofCandidate, 0)
	now := time.Now()
	cutoff := now.Add(-e.cfg.Event.Interval / 2)

	e.stateMu.RLock()
	defer e.stateMu.RUnlock()

	for deviceID, state := range e.states {
		if !e.isValidCheckInDevice(action, deviceID, state) {
			continue
		}
		roomID := *state.Session.RoomID
		e.collectSchulhofStudents(&candidates, deviceID, state, roomID, now, cutoff)
	}
	return candidates
}

// collectSchulhofStudents adds eligible students from a device to the Schulhof candidates list.
func (e *Engine) collectSchulhofStudents(candidates *[]schulhofCandidate, deviceID string, state *DeviceState, roomID int64, now, cutoff time.Time) {
	for _, student := range state.StudentStates {
		if student == nil || student.RFIDTag == "" {
			continue
		}

		// Check for Schulhof checkout candidates
		if e.isEligibleForSchulhofCheckout(student, now, cutoff) {
			*candidates = append(*candidates, schulhofCandidate{
				deviceID:    deviceID,
				studentID:   student.StudentID,
				studentRFID: student.RFIDTag,
				roomID:      roomID,
				apiAction:   "checkout",
			})
			continue
		}

		// Check for Schulhof checkin candidates
		if e.isEligibleForSchulhofCheckin(student, now) {
			*candidates = append(*candidates, schulhofCandidate{
				deviceID:    deviceID,
				studentID:   student.StudentID,
				studentRFID: student.RFIDTag,
				roomID:      roomID,
				apiAction:   "checkin",
			})
		}
	}
}

// isEligibleForSchulhofCheckout checks if a student is eligible for Schulhof checkout.
func (e *Engine) isEligibleForSchulhofCheckout(student *StudentState, now, cutoff time.Time) bool {
	if student.CurrentPhase != RotationPhaseSchulhof || student.CurrentRoomID == nil {
		return false
	}
	if !student.HasActiveVisit {
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

// isEligibleForSchulhofCheckin checks if a student is eligible for Schulhof checkin.
func (e *Engine) isEligibleForSchulhofCheckin(student *StudentState, now time.Time) bool {
	if student.NextPhase != RotationPhaseSchulhof || student.CurrentRoomID != nil {
		return false
	}
	if student.HasActiveVisit {
		return false
	}
	if !student.VisitCooldownUntil.IsZero() && student.VisitCooldownUntil.After(now) {
		return false
	}
	return true
}

// handleSchulhofError updates student state when Schulhof action fails.
func (e *Engine) handleSchulhofError(selected schulhofCandidate, payload CheckActionPayload, err error) error {
	missingVisit := payload.Action == "checkout" && isVisitMissingError(err)

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

// updateStateAfterSchulhofHop updates student state after successful Schulhof action.
func (e *Engine) updateStateAfterSchulhofHop(selected schulhofCandidate) {
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

	if selected.apiAction == "checkin" {
		e.applySchulhofCheckinState(student, selected.roomID, eventTime)
	} else {
		e.applySchulhofCheckoutState(student, eventTime)
	}

	if logging.Logger != nil {
		logging.Logger.WithFields(map[string]interface{}{
			"action":     "schulhof_" + selected.apiAction,
			"device_id":  selected.deviceID,
			"student_id": selected.studentID,
		}).Debug("Schulhof action completed")
	}
}

// applySchulhofCheckinState applies state changes for Schulhof check-in.
func (e *Engine) applySchulhofCheckinState(student *StudentState, roomID int64, eventTime time.Time) {
	student.CurrentRoomID = ptrInt64(roomID)
	student.CurrentPhase = RotationPhaseSchulhof
	student.NextPhase = RotationPhaseHeimatraum
	student.LastEventAt = eventTime
	student.HasActiveVisit = true
	student.VisitCooldownUntil = eventTime.Add(visitCooldown)
}

// applySchulhofCheckoutState applies state changes for Schulhof check-out.
func (e *Engine) applySchulhofCheckoutState(student *StudentState, eventTime time.Time) {
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
