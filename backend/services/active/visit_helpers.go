package active

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// WebManualDeviceCode is the device_id for manual web check-ins.
// This virtual device is created during seeding and represents check-ins
// performed through the web portal rather than physical RFID scanners.
const WebManualDeviceCode = iotModels.WebManualDeviceID

// ensureStudentHasNoActiveVisit checks that the student doesn't already have an active visit
func (s *service) ensureStudentHasNoActiveVisit(ctx context.Context, studentID int64) error {
	visits, err := s.SchoolPresence.ListVisits(ctx, studentpresence.VisitFilter{
		StudentIDs: []int64{studentID}, OpenOnly: true, Limit: 1,
	})
	if err != nil {
		return &ActiveError{Op: "CreateVisit", Err: ErrDatabaseOperation}
	}
	if len(visits) > 0 {
		return &ActiveError{Op: "CreateVisit", Err: ErrStudentAlreadyActive}
	}
	return nil
}

// resolveStaffIDForAttendance resolves the staff ID for attendance tracking
func (s *service) resolveStaffIDForAttendance(ctx context.Context, staffID, deviceID int64) (int64, error) {
	if staffID > 0 {
		return staffID, nil
	}
	if deviceID > 0 {
		supervisorID, err := s.getDeviceSupervisorID(ctx, deviceID)
		if err == nil {
			return supervisorID, nil
		}
		var unavailable *deviceSupervisorUnavailableError
		if !errors.As(err, &unavailable) {
			return 0, fmt.Errorf("resolve check-in staff attribution: %w", err)
		}
	}
	return 0, nil
}

// ensureOrUpdateAttendance handles attendance creation or re-entry update
func (s *service) ensureOrUpdateAttendance(ctx context.Context, visit *studentpresence.Visit, staffID, deviceID int64) error {
	visitDate := timezone.DateFromTime(visit.EntryTime)
	attendanceRecords, err := s.SchoolPresence.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{visit.StudentID}, FromDate: visitDate.String(), UntilDate: visitDate.String()})
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
func (s *service) createAttendanceRecord(ctx context.Context, visit *studentpresence.Visit, staffID, deviceID int64, visitDate timezone.Date) error {
	resolvedStaffID, err := s.resolveStaffIDForAttendance(ctx, staffID, deviceID)
	if err != nil {
		return &ActiveError{Op: "CreateVisit", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	resolvedDeviceID, err := s.resolveDeviceIDForAttendance(ctx, deviceID)
	if err != nil {
		return &ActiveError{Op: "CreateVisit", Err: errors.Join(ErrDatabaseOperation, err)}
	}

	attendance := studentpresence.Attendance{
		StudentID:   visit.StudentID,
		Date:        visitDate.String(),
		CheckInTime: visit.EntryTime,
		CheckedInBy: resolvedStaffID,
		DeviceID:    resolvedDeviceID,
	}
	if visit.ExitTime != nil {
		attendance.CheckOutTime = visit.ExitTime
		attendance.CheckedOutBy = &resolvedStaffID
	}

	attendance.TenantID = tenant.FromContext(ctx)
	if _, _, err := s.SchoolPresence.EnsureAttendance(ctx, attendance); err != nil {
		return &ActiveError{Op: "CreateVisit", Err: err}
	}
	return nil
}

// syncAttendanceForVisitRevision keeps the matching daily attendance session
// aligned with a visit edit. Entry times are stamped from the same source when
// visits are created, so the previous entry time is the session identity.
func (s *service) syncAttendanceForVisitRevision(
	ctx context.Context, previous, updated *studentpresence.Visit,
) error {
	if s.SchoolPresence == nil || previous == nil || updated == nil || previous.StudentID != updated.StudentID {
		return nil
	}
	if err := s.SchoolPresence.LockStudentAttendance(ctx, previous.StudentID); err != nil {
		return err
	}
	day := timezone.DateFromTime(previous.EntryTime).String()
	rows, err := s.SchoolPresence.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{previous.StudentID}, FromDate: day, UntilDate: day})
	if err != nil {
		return err
	}
	for _, row := range rows {
		if !row.CheckInTime.Equal(previous.EntryTime) {
			continue
		}
		row.Date = timezone.DateFromTime(updated.EntryTime).String()
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
		_, err := s.SchoolPresence.ReviseAttendance(ctx, row)
		return err
	}
	return nil
}

// resolveDeviceIDForAttendance resolves the device ID for attendance tracking.
// For manual web check-ins (deviceID == 0), it looks up the virtual web device.
func (s *service) resolveDeviceIDForAttendance(ctx context.Context, deviceID int64) (int64, error) {
	if deviceID > 0 {
		return deviceID, nil
	}

	// Look up the web manual device for manual check-ins
	webDevice, err := s.DeviceRepo.FindByDeviceID(ctx, WebManualDeviceCode)
	if err != nil {
		return 0, fmt.Errorf("resolve web manual device: %w", err)
	}
	if webDevice == nil {
		return 0, fmt.Errorf("resolve web manual device: %s is not configured", WebManualDeviceCode)
	}
	return webDevice.ID, nil
}

// resolveClearMode uses the tenant value or registry default supplied by settings.
func (s *service) resolveClearMode(ctx context.Context, key string) (string, error) {
	if s.settings == nil {
		return "", fmt.Errorf("resolve %s: settings service is not configured", key)
	}
	value, err := s.settings.ResolveString(ctx, key)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", key, err)
	}
	return value, nil
}

// autoClearStudentSickness clears the sickness flag on student check-in when
// the tenant's operations.sick_clear_mode setting is "next_checkin" (default).
func (s *service) autoClearStudentSickness(ctx context.Context, studentID int64) error {
	mode, err := s.resolveClearMode(ctx, configModel.KeySickClearMode)
	if err != nil {
		return err
	}
	if mode != configModel.ClearModeNextCheckin {
		return nil
	}

	student, err := s.StudentRepo.FindByID(ctx, studentID)
	if err != nil {
		return fmt.Errorf("load student for status clear: %w", err)
	}
	if student == nil {
		return ErrStudentNotFound
	}

	return s.clearSickFlagOnCheckin(ctx, student, time.Now())
}

// clearSickFlagOnCheckin is the write core of autoClearStudentSickness,
// operating on an already-loaded student so batch callers holding the row
// lock don't re-read per child (review #2372). No-op when the flag is unset.
func (s *service) clearSickFlagOnCheckin(ctx context.Context, student *userModels.Student, now time.Time) error {
	if student.Sick == nil || !*student.Sick {
		return nil
	}

	if err := s.recordStudentStatusForClear(ctx, student.ID, active.StudentStatusDaySick, student.SickSince, now, active.StudentStatusSourceNextCheckin); err != nil {
		return err
	}

	falseVal := false
	student.Sick = &falseVal
	student.SickSince = nil

	if err := s.StudentRepo.Update(ctx, student); err != nil {
		return fmt.Errorf("clear student status: %w", err)
	}

	s.getLogger().Info("auto-cleared sickness on student check-in",
		slog.Int64("student_id", student.ID),
	)
	return nil
}

// autoClearStudentExcused clears the excused flag on student check-in when
// the tenant's operations.excused_clear_mode setting is "next_checkin".
func (s *service) autoClearStudentExcused(ctx context.Context, studentID int64) error {
	mode, err := s.resolveClearMode(ctx, configModel.KeyExcusedClearMode)
	if err != nil {
		return err
	}
	if mode != configModel.ClearModeNextCheckin {
		return nil
	}

	student, err := s.StudentRepo.FindByID(ctx, studentID)
	if err != nil {
		return fmt.Errorf("load student for status clear: %w", err)
	}
	if student == nil {
		return ErrStudentNotFound
	}

	return s.clearExcusedFlagOnCheckin(ctx, student, time.Now())
}

// clearExcusedFlagOnCheckin is the write core of autoClearStudentExcused —
// same already-loaded-student contract as clearSickFlagOnCheckin.
func (s *service) clearExcusedFlagOnCheckin(ctx context.Context, student *userModels.Student, now time.Time) error {
	if student.Excused == nil || !*student.Excused {
		return nil
	}

	if err := s.recordStudentStatusForClear(ctx, student.ID, active.StudentStatusDayExcused, student.ExcusedSince, now, active.StudentStatusSourceNextCheckin); err != nil {
		return err
	}

	falseVal := false
	student.Excused = &falseVal
	student.ExcusedSince = nil

	if err := s.StudentRepo.Update(ctx, student); err != nil {
		return fmt.Errorf("clear student status: %w", err)
	}

	s.getLogger().Info("auto-cleared excused on student check-in",
		slog.Int64("student_id", student.ID),
	)
	return nil
}

func (s *service) recordStudentStatusForClear(ctx context.Context, studentID int64, status string, since *time.Time, now time.Time, source string) error {
	if s.StudentStatusRepo == nil {
		return nil
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
		return fmt.Errorf("clear student status: %w", err)
	}
	if err := s.StudentStatusRepo.MarkCleared(ctx, studentID, status, today, now, source); err != nil {
		return fmt.Errorf("clear student status history: %w", err)
	}
	return nil
}

func (s *service) autoClearPlannedStudentStatuses(ctx context.Context, studentID int64) error {
	if s.StudentStatusRepo == nil {
		return nil
	}

	now := s.now()
	today := timezone.DateFromTime(now)
	rows, err := s.StudentStatusRepo.FindActiveByStudentAndDateRange(ctx, studentID, today, today)
	if err != nil {
		return fmt.Errorf("load planned student statuses: %w", err)
	}

	return s.clearPlannedStatusRows(ctx, studentID, nil, rows, now)
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
) error {
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
			return fmt.Errorf("clear planned student status: %w", err)
		}
		if row.Status == active.StudentStatusDaySick {
			hasPlannedSick = true
		}
		if row.Status == active.StudentStatusDayExcused || row.Status == active.StudentStatusDayClassTrip {
			hasPlannedExcused = true
		}
	}

	if !hasPlannedSick && !hasPlannedExcused {
		return nil
	}

	if student == nil {
		loaded, err := s.StudentRepo.FindByID(ctx, studentID)
		if err != nil {
			return fmt.Errorf("load student for planned status clear: %w", err)
		}
		if loaded == nil {
			return ErrStudentNotFound
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
		return fmt.Errorf("clear planned student flags: %w", err)
	}
	return nil
}

// broadcastVisitCreated sends SSE event for visit creation.
// snapshot (WP-B10) may be nil — when present, it enriches the event with
// attendance_status/substatus/note so subscribers see the flipped attendance
// state alongside the check-in line.
func (s *service) broadcastVisitCreated(ctx context.Context, visit *studentpresence.Visit, snapshot *AttendanceSnapshot) {
	// Der Raum-Check-in der detaillierten Betriebsart schreibt seine eigene
	// Anwesenheitszeile und laeuft NICHT ueber registerCheckinBroadcast, also
	// weckt er die Sorgeberechtigten hier selbst.
	s.wakeGuardiansAfterCommit(ctx, visit.StudentID)

	if s.Broadcaster == nil {
		return
	}
	studentRec := s.getStudentForSSE(ctx, visit.StudentID)
	broadcastCtx := tenant.ContextWithoutTransaction(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		s.emitVisitCreated(broadcastCtx, visit, snapshot, studentRec)
	})
}

// emitVisitCreated publishes a visit using routing data already resolved in
// the request transaction. Move events reuse the same student record for their
// checkout and check-in halves.
func (s *service) emitVisitCreated(ctx context.Context, visit *studentpresence.Visit, snapshot *AttendanceSnapshot, studentRec *userModels.Student) {
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

	// One precise tenant event replaces the old dashboard_counts_changed +
	// active_supervision_changed pair.
	s.broadcastSupervisionRefresh(ctx, activeGroupID, activeSupervisionReasonStudentMoved, eduGroupIDs)
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
