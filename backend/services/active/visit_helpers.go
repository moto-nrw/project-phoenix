package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/models/active"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// WebManualDeviceCode is the device_id for manual web check-ins.
// This virtual device is created during seeding and represents check-ins
// performed through the web portal rather than physical RFID scanners.
const WebManualDeviceCode = "WEB-MANUAL-001"

// ensureStudentHasNoActiveVisit checks that the student doesn't already have an active visit
func (s *service) ensureStudentHasNoActiveVisit(ctx context.Context, studentID int64) error {
	visits, err := s.visitRepo.FindActiveByStudentID(ctx, studentID)
	if err != nil {
		return &ActiveError{Op: "CreateVisit", Err: ErrDatabaseOperation}
	}
	if len(visits) > 0 {
		return &ActiveError{Op: "CreateVisit", Err: ErrStudentAlreadyActive}
	}
	return nil
}

// resolveStaffIDForAttendance resolves the staff ID for attendance tracking
func (s *service) resolveStaffIDForAttendance(ctx context.Context, staffID, deviceID int64) int64 {
	if staffID > 0 {
		return staffID
	}
	if deviceID > 0 {
		if supervisorID, err := s.getDeviceSupervisorID(ctx, deviceID); err == nil {
			return supervisorID
		}
	}
	return 0
}

// ensureOrUpdateAttendance handles attendance creation or re-entry update
func (s *service) ensureOrUpdateAttendance(ctx context.Context, visit *active.Visit, staffID, deviceID int64) error {
	// Use UTC date for attendance tracking to avoid timezone conversion issues.
	// The repository's FindByStudentAndDate uses date.Truncate(24 * time.Hour) which
	// truncates in UTC, so we need to match that here.
	visitDate := visit.EntryTime.UTC().Truncate(24 * time.Hour)
	attendanceRecords, err := s.attendanceRepo.FindByStudentAndDate(ctx, visit.StudentID, visitDate)
	if err != nil {
		return &ActiveError{Op: "CreateVisit", Err: err}
	}

	if len(attendanceRecords) == 0 {
		return s.createAttendanceRecord(ctx, visit, staffID, deviceID, visitDate)
	}

	// Attendance exists - handle re-entry case
	s.clearCheckoutOnReentry(ctx, visit.StudentID, attendanceRecords)
	return nil
}

// createAttendanceRecord creates a new attendance record for first visit of the day
func (s *service) createAttendanceRecord(ctx context.Context, visit *active.Visit, staffID, deviceID int64, visitDate time.Time) error {
	resolvedStaffID := s.resolveStaffIDForAttendance(ctx, staffID, deviceID)
	resolvedDeviceID := s.resolveDeviceIDForAttendance(ctx, deviceID)

	attendance := &active.Attendance{
		StudentID:   visit.StudentID,
		Date:        visitDate,
		CheckInTime: visit.EntryTime,
		CheckedInBy: resolvedStaffID,
		DeviceID:    resolvedDeviceID,
	}

	if err := s.attendanceRepo.Create(ctx, attendance); err != nil {
		return &ActiveError{Op: "CreateVisit", Err: err}
	}
	return nil
}

// resolveDeviceIDForAttendance resolves the device ID for attendance tracking.
// For manual web check-ins (deviceID == 0), it looks up the virtual web device.
func (s *service) resolveDeviceIDForAttendance(ctx context.Context, deviceID int64) int64 {
	if deviceID > 0 {
		return deviceID
	}

	// Look up the web manual device for manual check-ins
	webDevice, err := s.deviceRepo.FindByDeviceID(ctx, WebManualDeviceCode)
	if err == nil && webDevice != nil {
		return webDevice.ID
	}

	// Log warning if web device not found - this indicates a seeding issue
	if logging.Logger != nil {
		logging.Logger.WithFields(map[string]interface{}{
			"device_code": WebManualDeviceCode,
			"error":       err,
		}).Warn("Web manual device not found - manual check-ins may fail")
	}

	return 0
}

// clearCheckoutOnReentry clears checkout time for re-entry after daily checkout
func (s *service) clearCheckoutOnReentry(ctx context.Context, studentID int64, attendanceRecords []*active.Attendance) {
	for _, attendance := range attendanceRecords {
		if attendance.CheckOutTime == nil {
			continue
		}

		attendance.CheckOutTime = nil
		attendance.CheckedOutBy = nil
		if err := s.attendanceRepo.Update(ctx, attendance); err != nil {
			logAttendanceUpdateFailure(studentID, attendance.ID, err)
		}
	}
}

// logAttendanceUpdateFailure logs a warning when attendance update fails
func logAttendanceUpdateFailure(studentID, attendanceID int64, err error) {
	if logging.Logger == nil {
		return
	}
	logging.Logger.WithFields(map[string]interface{}{
		"student_id":    studentID,
		"attendance_id": attendanceID,
		"error":         err.Error(),
	}).Warn("Failed to clear check_out_time on re-entry")
}

// autoClearStudentSickness clears sickness flag when student checks in
func (s *service) autoClearStudentSickness(ctx context.Context, studentID int64) {
	student, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil || student == nil {
		return
	}

	if student.Sick == nil || !*student.Sick {
		return
	}

	// Student is marked as sick, clear it since they're checking in
	falseVal := false
	student.Sick = &falseVal
	student.SickSince = nil

	if err := s.studentRepo.Update(ctx, student); err != nil {
		logSicknessClearFailure(studentID, err)
		return
	}

	logSicknessClearSuccess(studentID)
}

// logSicknessClearFailure logs a warning when sickness clear fails
func logSicknessClearFailure(studentID int64, err error) {
	if logging.Logger == nil {
		return
	}
	logging.Logger.WithFields(map[string]interface{}{
		"student_id": studentID,
		"error":      err.Error(),
	}).Warn("Failed to auto-clear sickness on check-in")
}

// logSicknessClearSuccess logs successful sickness clear
func logSicknessClearSuccess(studentID int64) {
	if logging.Logger == nil {
		return
	}
	logging.Logger.WithFields(map[string]interface{}{
		"student_id": studentID,
	}).Info("Auto-cleared sickness on student check-in")
}

// broadcastVisitCreated sends SSE event for visit creation
func (s *service) broadcastVisitCreated(ctx context.Context, visit *active.Visit) {
	if s.broadcaster == nil {
		return
	}

	activeGroupID := fmt.Sprintf("%d", visit.ActiveGroupID)
	studentID := fmt.Sprintf("%d", visit.StudentID)

	studentName, studentRec := s.getStudentDisplayData(ctx, visit.StudentID)

	event := realtime.NewEvent(
		realtime.EventStudentCheckIn,
		activeGroupID,
		realtime.EventData{
			StudentID:   &studentID,
			StudentName: &studentName,
		},
	)

	if err := s.broadcaster.BroadcastToGroup(activeGroupID, event); err != nil {
		logSSEBroadcastFailure(activeGroupID, studentID, err)
	}

	s.broadcastToEducationalGroup(studentRec, event)
}

// getStudentDisplayData fetches student name for display
func (s *service) getStudentDisplayData(ctx context.Context, studentID int64) (string, *userModels.Student) {
	student, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil || student == nil {
		return "", nil
	}

	person, err := s.personRepo.FindByID(ctx, student.PersonID)
	if err != nil || person == nil {
		return "", student
	}

	return fmt.Sprintf("%s %s", person.FirstName, person.LastName), student
}

// logSSEBroadcastFailure logs SSE broadcast failure
func logSSEBroadcastFailure(activeGroupID, studentID string, err error) {
	if logging.Logger == nil {
		return
	}
	logging.Logger.WithFields(map[string]interface{}{
		"error":           err.Error(),
		"event_type":      "student_checkin",
		"active_group_id": activeGroupID,
		"student_id":      studentID,
	}).Error("SSE broadcast failed")
}
