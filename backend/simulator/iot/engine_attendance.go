package iot

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/attendance"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
)

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

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"device_id":  selected.deviceID,
			"student_id": selected.studentID,
			"status":     student.AttendanceStatus,
		}).Debug("Attendance toggle completed")
	}
}
