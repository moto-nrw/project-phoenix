package active

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// WebManualDeviceCode is the device_id for manual web check-ins.
// This virtual device is created during seeding and represents check-ins
// performed through the web portal rather than physical RFID scanners.
const WebManualDeviceCode = iotModels.WebManualDeviceID

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
	// Use DateOfUTC: extract the Berlin calendar date but return as UTC midnight,
	// so the value round-trips correctly through PostgreSQL DATE columns (PG session is UTC).
	visitDate := timezone.DateOfUTC(visit.EntryTime)
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

	attendance.SetTenantID(tenant.FromContext(ctx))
	if _, err := s.attendanceRepo.CreateIfNoOpenForToday(ctx, attendance); err != nil {
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
	s.getLogger().Warn("web manual device not found - manual check-ins may fail",
		slog.String("device_code", WebManualDeviceCode),
		slog.Any("error", err),
	)

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
			s.getLogger().Warn("failed to clear check_out_time on re-entry",
				slog.Int64("student_id", studentID),
				slog.Int64("attendance_id", attendance.ID),
				slog.String("error", err.Error()),
			)
		}
	}
}

// resolveClearMode resolves the configured clear mode for a status flag,
// falling back to the provided default when the settings resolver is unavailable
// or has no tenant override. Registry defaults are populated via SetValue flow,
// not via Resolve* — so we check HasTenantOverride explicitly.
func (s *service) resolveClearMode(ctx context.Context, key, fallback string) string {
	if s.settings == nil {
		return fallback
	}
	hasOverride, err := s.settings.HasTenantOverride(ctx, key)
	if err != nil {
		s.getLogger().Warn("settings override check failed, using default",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return fallback
	}
	if !hasOverride {
		return fallback
	}
	val, err := s.settings.ResolveString(ctx, key)
	if err != nil || val == "" {
		return fallback
	}
	return val
}

// autoClearStudentSickness clears the sickness flag on student check-in when
// the tenant's operations.sick_clear_mode setting is "next_checkin" (default).
func (s *service) autoClearStudentSickness(ctx context.Context, studentID int64) {
	mode := s.resolveClearMode(ctx, configModel.KeySickClearMode, configModel.ClearModeNextCheckin)
	if mode != configModel.ClearModeNextCheckin {
		return
	}

	student, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil || student == nil {
		return
	}

	if student.Sick == nil || !*student.Sick {
		return
	}

	now := time.Now()
	s.recordStudentStatusForClear(ctx, studentID, active.StudentStatusDaySick, student.SickSince, now, active.StudentStatusSourceNextCheckin)

	falseVal := false
	student.Sick = &falseVal
	student.SickSince = nil

	if err := s.studentRepo.Update(ctx, student); err != nil {
		s.getLogger().Warn("failed to auto-clear sickness on check-in",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return
	}

	s.getLogger().Info("auto-cleared sickness on student check-in",
		slog.Int64("student_id", studentID),
	)
}

// autoClearStudentExcused clears the excused flag on student check-in when
// the tenant's operations.excused_clear_mode setting is "next_checkin".
func (s *service) autoClearStudentExcused(ctx context.Context, studentID int64) {
	mode := s.resolveClearMode(ctx, configModel.KeyExcusedClearMode, configModel.ClearModeEndOfDay)
	if mode != configModel.ClearModeNextCheckin {
		return
	}

	student, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil || student == nil {
		return
	}

	if student.Excused == nil || !*student.Excused {
		return
	}

	now := time.Now()
	s.recordStudentStatusForClear(ctx, studentID, active.StudentStatusDayExcused, student.ExcusedSince, now, active.StudentStatusSourceNextCheckin)

	falseVal := false
	student.Excused = &falseVal
	student.ExcusedSince = nil

	if err := s.studentRepo.Update(ctx, student); err != nil {
		s.getLogger().Warn("failed to auto-clear excused on check-in",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return
	}

	s.getLogger().Info("auto-cleared excused on student check-in",
		slog.Int64("student_id", studentID),
	)
}

func (s *service) recordStudentStatusForClear(ctx context.Context, studentID int64, status string, since *time.Time, now time.Time, source string) {
	if s.studentStatusRepo == nil {
		return
	}
	reportedAt := now
	if since != nil {
		reportedAt = *since
	}
	today := timezone.DateOfUTC(now)
	if err := s.studentStatusRepo.UpsertReported(ctx, &active.StudentStatusDay{
		StudentID:  studentID,
		Date:       today,
		Status:     status,
		ReportedAt: reportedAt,
		Source:     source,
	}); err != nil {
		s.getLogger().Warn("failed to record student status before auto-clear",
			slog.Int64("student_id", studentID),
			slog.String("status", status),
			slog.String("error", err.Error()),
		)
		return
	}
	if err := s.studentStatusRepo.MarkCleared(ctx, studentID, status, today, now, source); err != nil {
		s.getLogger().Warn("failed to close student status history on auto-clear",
			slog.Int64("student_id", studentID),
			slog.String("status", status),
			slog.String("error", err.Error()),
		)
	}
}

func (s *service) autoClearPlannedStudentStatuses(ctx context.Context, studentID int64) {
	if s.studentStatusRepo == nil {
		return
	}

	now := time.Now()
	today := timezone.DateOfUTC(now)
	rows, err := s.studentStatusRepo.FindActiveByStudentAndDateRange(ctx, studentID, today, today)
	if err != nil {
		s.getLogger().Warn("failed to load planned student status days on check-in",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return
	}

	hasPlannedSick := false
	hasPlannedExcused := false
	for _, row := range rows {
		if row.Source != active.StudentStatusSourcePlanned {
			continue
		}
		if err := s.studentStatusRepo.MarkClearedByID(ctx, row.ID, now, active.StudentStatusSourceNextCheckin); err != nil {
			s.getLogger().Warn("failed to clear planned student status day on check-in",
				slog.Int64("student_id", studentID),
				slog.String("status", row.Status),
				slog.String("error", err.Error()),
			)
			continue
		}
		if row.Status == active.StudentStatusDaySick {
			hasPlannedSick = true
		}
		if row.Status == active.StudentStatusDayExcused {
			hasPlannedExcused = true
		}
	}

	if !hasPlannedSick && !hasPlannedExcused {
		return
	}

	student, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil || student == nil {
		return
	}

	falseVal := false
	if hasPlannedSick {
		student.Sick = &falseVal
		student.SickSince = nil
	}
	if hasPlannedExcused {
		student.Excused = &falseVal
		student.ExcusedSince = nil
	}
	if err := s.studentRepo.Update(ctx, student); err != nil {
		s.getLogger().Warn("failed to clear planned student flags on check-in",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
	}
}

// broadcastVisitCreated sends SSE event for visit creation.
// snapshot (WP-B10) may be nil — when present, it enriches the event with
// attendance_status/substatus/note so subscribers see the flipped attendance
// state alongside the check-in line.
func (s *service) broadcastVisitCreated(ctx context.Context, visit *active.Visit, snapshot *AttendanceSnapshot) {
	if s.broadcaster == nil {
		return
	}

	activeGroupID := fmt.Sprintf("%d", visit.ActiveGroupID)
	studentID := fmt.Sprintf("%d", visit.StudentID)

	studentName, studentRec := s.getStudentDisplayData(ctx, visit.StudentID)

	data := realtime.EventData{
		StudentID:   &studentID,
		StudentName: &studentName,
	}
	applyAttendanceSnapshot(&data, snapshot)

	event := realtime.NewEvent(
		realtime.EventStudentCheckIn,
		activeGroupID,
		data,
	)

	if err := s.broadcaster.BroadcastToGroup(tenant.FromContext(ctx), activeGroupID, event); err != nil {
		s.getLogger().Error("SSE broadcast failed",
			slog.String("error", err.Error()),
			slog.String("event_type", "student_checkin"),
			slog.String("active_group_id", activeGroupID),
			slog.String("student_id", studentID),
		)
	}

	s.broadcastToEducationalGroup(ctx, studentRec, event)

	// Notify all clients so dashboard counts refresh
	_ = s.broadcaster.BroadcastToAll(realtime.NewEvent(realtime.EventDashboardCountsChanged, "", realtime.EventData{}))
	s.broadcastActiveSupervisionChanged(ctx, activeGroupID, studentID, activeSupervisionReasonStudentMoved)
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
