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
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// WebManualDeviceCode is the device_id for manual web check-ins.
// This virtual device is created during seeding and represents check-ins
// performed through the web portal rather than physical RFID scanners.
const WebManualDeviceCode = iotModels.WebManualDeviceID

// ensureStudentHasNoActiveVisit checks that the student doesn't already have an active visit
func (s *service) ensureStudentHasNoActiveVisit(ctx context.Context, studentID int64) error {
	visits, err := s.VisitRepo.FindActiveByStudentID(ctx, studentID)
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
	visitDate := timezone.DateFromTime(visit.EntryTime)
	attendanceRecords, err := s.AttendanceRepo.FindByStudentAndDate(ctx, visit.StudentID, visitDate)
	if err != nil {
		return &ActiveError{Op: "CreateVisit", Err: err}
	}

	for _, attendance := range attendanceRecords {
		if attendance.CheckOutTime == nil {
			return nil
		}
	}

	// Every completed stay remains immutable. A later return creates a new
	// session instead of reopening and corrupting the earlier checkout.
	return s.createAttendanceRecord(ctx, visit, staffID, deviceID, visitDate)
}

// createAttendanceRecord creates a new attendance record for first visit of the day.
// CheckInTime is deliberately visit.EntryTime — the slot-attendance mirror
// (schedule.AttendanceSyncService) stamps instance_students.checked_in_at from
// the same instant, and history/export session-to-slot matching relies on the
// two timestamps being identical. Never replace either side with an
// independent time.Now().
func (s *service) createAttendanceRecord(ctx context.Context, visit *active.Visit, staffID, deviceID int64, visitDate timezone.Date) error {
	resolvedStaffID := s.resolveStaffIDForAttendance(ctx, staffID, deviceID)
	resolvedDeviceID := s.resolveDeviceIDForAttendance(ctx, deviceID)

	attendance := &active.Attendance{
		StudentID:   visit.StudentID,
		Date:        visitDate,
		CheckInTime: visit.EntryTime,
		CheckedInBy: resolvedStaffID,
		DeviceID:    resolvedDeviceID,
	}
	if visit.ExitTime != nil {
		attendance.CheckOutTime = visit.ExitTime
		attendance.CheckedOutBy = &resolvedStaffID
	}

	attendance.SetTenantID(tenant.FromContext(ctx))
	if _, err := s.AttendanceRepo.CreateIfNoOpenForToday(ctx, attendance); err != nil {
		return &ActiveError{Op: "CreateVisit", Err: err}
	}
	return nil
}

// syncAttendanceForVisitRevision keeps the matching daily attendance session
// aligned with a visit edit. Entry times are stamped from the same source when
// visits are created, so the previous entry time is the session identity.
func (s *service) syncAttendanceForVisitRevision(
	ctx context.Context, previous, updated *active.Visit,
) error {
	if s.AttendanceRepo == nil || previous == nil || updated == nil || previous.StudentID != updated.StudentID {
		return nil
	}
	if err := s.AttendanceRepo.LockStudentAttendance(ctx, previous.StudentID); err != nil {
		return err
	}
	rows, err := s.AttendanceRepo.FindByStudentAndDate(
		ctx, previous.StudentID, timezone.DateFromTime(previous.EntryTime),
	)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row == nil || !row.CheckInTime.Equal(previous.EntryTime) {
			continue
		}
		row.Date = timezone.DateFromTime(updated.EntryTime)
		row.CheckInTime = updated.EntryTime
		row.CheckOutTime = updated.ExitTime
		if updated.ExitTime == nil {
			row.CheckedOutBy = nil
		} else if row.CheckedOutBy == nil {
			_, staffID := s.extractContextIDs(ctx)
			if staffID > 0 {
				row.CheckedOutBy = &staffID
			}
		}
		return s.AttendanceRepo.Update(ctx, row)
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
	webDevice, err := s.DeviceRepo.FindByDeviceID(ctx, WebManualDeviceCode)
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

// resolveClearMode resolves the configured clear mode for a status flag,
// falling back to the provided default when the settings resolver is unavailable
// or has no tenant override.
func (s *service) resolveClearMode(ctx context.Context, key, fallback string) string {
	return config.ResolveStringOrDefault(ctx, s.settings, key, fallback, s.getLogger())
}

// autoClearStudentSickness clears the sickness flag on student check-in when
// the tenant's operations.sick_clear_mode setting is "next_checkin" (default).
func (s *service) autoClearStudentSickness(ctx context.Context, studentID int64) {
	mode := s.resolveClearMode(ctx, configModel.KeySickClearMode, configModel.ClearModeNextCheckin)
	if mode != configModel.ClearModeNextCheckin {
		return
	}

	student, err := s.StudentRepo.FindByID(ctx, studentID)
	if err != nil || student == nil {
		return
	}

	s.clearSickFlagOnCheckin(ctx, student, time.Now())
}

// clearSickFlagOnCheckin is the write core of autoClearStudentSickness,
// operating on an already-loaded student so batch callers holding the row
// lock don't re-read per child (review #2372). No-op when the flag is unset.
func (s *service) clearSickFlagOnCheckin(ctx context.Context, student *userModels.Student, now time.Time) {
	if student.Sick == nil || !*student.Sick {
		return
	}

	s.recordStudentStatusForClear(ctx, student.ID, active.StudentStatusDaySick, student.SickSince, now, active.StudentStatusSourceNextCheckin)

	falseVal := false
	student.Sick = &falseVal
	student.SickSince = nil

	if err := s.StudentRepo.Update(ctx, student); err != nil {
		s.getLogger().Warn("failed to auto-clear sickness on check-in",
			slog.Int64("student_id", student.ID),
			slog.String("error", err.Error()),
		)
		return
	}

	s.getLogger().Info("auto-cleared sickness on student check-in",
		slog.Int64("student_id", student.ID),
	)
}

// autoClearStudentExcused clears the excused flag on student check-in when
// the tenant's operations.excused_clear_mode setting is "next_checkin".
func (s *service) autoClearStudentExcused(ctx context.Context, studentID int64) {
	mode := s.resolveClearMode(ctx, configModel.KeyExcusedClearMode, configModel.ClearModeEndOfDay)
	if mode != configModel.ClearModeNextCheckin {
		return
	}

	student, err := s.StudentRepo.FindByID(ctx, studentID)
	if err != nil || student == nil {
		return
	}

	s.clearExcusedFlagOnCheckin(ctx, student, time.Now())
}

// clearExcusedFlagOnCheckin is the write core of autoClearStudentExcused —
// same already-loaded-student contract as clearSickFlagOnCheckin.
func (s *service) clearExcusedFlagOnCheckin(ctx context.Context, student *userModels.Student, now time.Time) {
	if student.Excused == nil || !*student.Excused {
		return
	}

	s.recordStudentStatusForClear(ctx, student.ID, active.StudentStatusDayExcused, student.ExcusedSince, now, active.StudentStatusSourceNextCheckin)

	falseVal := false
	student.Excused = &falseVal
	student.ExcusedSince = nil

	if err := s.StudentRepo.Update(ctx, student); err != nil {
		s.getLogger().Warn("failed to auto-clear excused on check-in",
			slog.Int64("student_id", student.ID),
			slog.String("error", err.Error()),
		)
		return
	}

	s.getLogger().Info("auto-cleared excused on student check-in",
		slog.Int64("student_id", student.ID),
	)
}

func (s *service) recordStudentStatusForClear(ctx context.Context, studentID int64, status string, since *time.Time, now time.Time, source string) {
	if s.StudentStatusRepo == nil {
		return
	}
	reportedAt := now
	if since != nil {
		reportedAt = *since
	}
	today := timezone.DateFromTime(now)
	if err := s.StudentStatusRepo.UpsertReported(ctx, &active.StudentStatusDay{
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
	if err := s.StudentStatusRepo.MarkCleared(ctx, studentID, status, today, now, source); err != nil {
		s.getLogger().Warn("failed to close student status history on auto-clear",
			slog.Int64("student_id", studentID),
			slog.String("status", status),
			slog.String("error", err.Error()),
		)
	}
}

func (s *service) autoClearPlannedStudentStatuses(ctx context.Context, studentID int64) {
	if s.StudentStatusRepo == nil {
		return
	}

	now := time.Now()
	today := timezone.DateFromTime(now)
	rows, err := s.StudentStatusRepo.FindActiveByStudentAndDateRange(ctx, studentID, today, today)
	if err != nil {
		s.getLogger().Warn("failed to load planned student status days on check-in",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return
	}

	s.clearPlannedStatusRows(ctx, studentID, nil, rows, now)
}

// clearPlannedStatusRows is the write core of autoClearPlannedStudentStatuses,
// operating on pre-fetched status-day rows so batch callers can load every
// student's rows in one query (review #2372). student may be nil — it is
// fetched lazily only when a flag actually needs clearing; batch callers pass
// their already-locked row to skip that read too.
func (s *service) clearPlannedStatusRows(
	ctx context.Context,
	studentID int64,
	student *userModels.Student,
	rows []*active.StudentStatusDay,
	now time.Time,
) {
	hasPlannedSick := false
	hasPlannedExcused := false
	for _, row := range rows {
		// Both staff-planned and parent-reported sick/excused days are
		// "scheduled ahead" rows the live-flag path doesn't cover, so the
		// next-checkin clear must treat them the same — otherwise a parent
		// sick note for today stays active even after the child checks in.
		if row.Source != active.StudentStatusSourcePlanned &&
			row.Source != active.StudentStatusSourceParent {
			continue
		}
		if err := s.StudentStatusRepo.MarkClearedByID(ctx, row.ID, now, active.StudentStatusSourceNextCheckin); err != nil {
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
		if row.Status == active.StudentStatusDayExcused || row.Status == active.StudentStatusDayClassTrip {
			hasPlannedExcused = true
		}
	}

	if !hasPlannedSick && !hasPlannedExcused {
		return
	}

	if student == nil {
		loaded, err := s.StudentRepo.FindByID(ctx, studentID)
		if err != nil || loaded == nil {
			return
		}
		student = loaded
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
	if err := s.StudentRepo.Update(ctx, student); err != nil {
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
	// Der Raum-Check-in der detaillierten Betriebsart schreibt seine eigene
	// Anwesenheitszeile und laeuft NICHT ueber registerCheckinBroadcast, also
	// weckt er die Sorgeberechtigten hier selbst.
	s.wakeGuardiansAfterCommit(ctx, visit.StudentID)

	if s.Broadcaster == nil {
		return
	}
	s.emitVisitCreated(ctx, visit, snapshot, s.getStudentForSSE(ctx, visit.StudentID))
}

// emitVisitCreated publishes a visit using routing data already resolved in
// the request transaction. Move events reuse the same student record for their
// checkout and check-in halves.
func (s *service) emitVisitCreated(ctx context.Context, visit *active.Visit, snapshot *AttendanceSnapshot, studentRec *userModels.Student) {
	if s.Broadcaster == nil || visit == nil {
		return
	}

	activeGroupID := fmt.Sprintf("%d", visit.ActiveGroupID)
	studentID := fmt.Sprintf("%d", visit.StudentID)

	eduGroupIDs := eduGroupIDsOf(studentRec)

	data := realtime.EventData{
		StudentID: &studentID,
	}
	if len(eduGroupIDs) > 0 {
		data.GroupIDs = &eduGroupIDs
	}
	applyAttendanceSnapshot(&data, snapshot)

	event := realtime.NewEvent(
		realtime.EventStudentCheckIn,
		activeGroupID,
		data,
	)

	if err := s.broadcastVisitEvent(ctx, activeGroupID, studentRec, event); err != nil {
		s.getLogger().Error("SSE broadcast failed",
			slog.String("error", err.Error()),
			slog.String("event_type", "student_checkin"),
			slog.String("active_group_id", activeGroupID),
			slog.String("student_id", studentID),
		)
	}
	s.broadcastRosterRefreshToTopics(ctx, activeGroupID, studentRec, eduGroupIDs)

	// One precise tenant event replaces the old dashboard_counts_changed +
	// active_supervision_changed pair.
	s.broadcastSupervisionRefresh(ctx, activeGroupID, activeSupervisionReasonStudentMoved, eduGroupIDs)
}

// broadcastRosterRefreshToTopics preserves active-supervision invalidation for
// an older frontend during a rolling deploy. It is group-scoped rather than
// tenant-wide, so only clients entitled to this roster pay for the compatibility
// frame; BroadcastToGroups still deduplicates clients subscribed to both topics.
func (s *service) broadcastRosterRefreshToTopics(ctx context.Context, activeGroupID string, studentRec *userModels.Student, eduGroupIDs []string) {
	reason := activeSupervisionReasonStudentMoved
	data := realtime.EventData{Reason: &reason}
	if len(eduGroupIDs) > 0 {
		data.GroupIDs = &eduGroupIDs
	}
	event := realtime.NewEvent(realtime.EventActiveSupervisionChanged, activeGroupID, data)
	if err := s.broadcastVisitEvent(ctx, activeGroupID, studentRec, event); err != nil {
		s.getLogger().Warn("SSE roster compatibility broadcast failed",
			slog.String("error", err.Error()),
			slog.String("active_group_id", activeGroupID),
		)
	}
}

func (s *service) broadcastVisitEvent(ctx context.Context, activeGroupID string, studentRec *userModels.Student, event realtime.Event) error {
	topics := []string{activeGroupID}
	if studentRec != nil && studentRec.GroupID != nil {
		topics = append(topics, fmt.Sprintf("edu:%d", *studentRec.GroupID))
	}
	return s.Broadcaster.BroadcastToGroups(tenant.FromContext(ctx), topics, event)
}

// getStudentForSSE resolves only routing data. Names are never consumed by SSE
// clients and used to cost an additional person query per attendance change.
func (s *service) getStudentForSSE(ctx context.Context, studentID int64) *userModels.Student {
	student, err := s.StudentRepo.FindByID(ctx, studentID)
	if err != nil || student == nil {
		return nil
	}
	return student
}
