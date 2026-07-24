package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Attendance tracking operations

func (s *service) GetStudentsAttendanceStatuses(ctx context.Context, studentIDs []int64) (map[int64]*AttendanceStatus, error) {
	if len(studentIDs) == 0 {
		return map[int64]*AttendanceStatus{}, nil
	}

	statuses := make(map[int64]*AttendanceStatus, len(studentIDs))

	attendanceRecords, err := s.AttendanceRepo.GetTodayByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetStudentsAttendanceStatuses", Err: ErrDatabaseOperation}
	}
	if attendanceRecords == nil {
		attendanceRecords = make(map[int64]*active.Attendance)
	}

	today := timezone.TodayDate()

	for _, studentID := range studentIDs {
		status := &AttendanceStatus{
			StudentID: studentID,
			Status:    "not_checked_in",
			Date:      today,
		}

		if attendance, ok := attendanceRecords[studentID]; ok && attendance != nil {
			status.Date = attendance.Date
			status.CheckInTime = &attendance.CheckInTime
			status.CheckOutTime = attendance.CheckOutTime
			status.YardSince = attendance.YardSince
			status.Status = deriveAttendanceStatus(attendance)
		}

		statuses[studentID] = status
	}

	return statuses, nil
}

// deriveAttendanceStatus turns the attendance row's timestamp combination into
// one of "checked_out", "on_yard", or "checked_in". Precedence: a checkout
// time always wins (the student has formally left school), then yard_since
// (on premises but outside the building), else the default checked_in.
func deriveAttendanceStatus(a *active.Attendance) string {
	if a.CheckOutTime != nil {
		return "checked_out"
	}
	if a.YardSince != nil {
		return "on_yard"
	}
	return "checked_in"
}

// GetStudentAttendanceStatus gets today's latest attendance record and determines status
func (s *service) GetStudentAttendanceStatus(ctx context.Context, studentID int64) (*AttendanceStatus, error) {
	attendance, err := s.AttendanceRepo.GetStudentCurrentStatus(ctx, studentID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, &ActiveError{Op: "GetStudentAttendanceStatus", Err: ErrDatabaseOperation}
		}
		return &AttendanceStatus{
			StudentID: studentID,
			Status:    "not_checked_in",
			Date:      timezone.TodayDate(),
		}, nil
	}

	result := &AttendanceStatus{
		StudentID:    studentID,
		Status:       deriveAttendanceStatus(attendance),
		Date:         attendance.Date,
		CheckInTime:  &attendance.CheckInTime,
		CheckOutTime: attendance.CheckOutTime,
		YardSince:    attendance.YardSince,
	}

	s.populateAttendanceStaffNames(ctx, result, attendance)
	return result, nil
}

// populateAttendanceStaffNames populates staff names for check-in and check-out
func (s *service) populateAttendanceStaffNames(ctx context.Context, result *AttendanceStatus, attendance *active.Attendance) {
	if attendance.CheckedInBy > 0 {
		result.CheckedInBy = s.getStaffNameByID(ctx, attendance.CheckedInBy)
	}

	if attendance.CheckedOutBy != nil && *attendance.CheckedOutBy > 0 {
		result.CheckedOutBy = s.getStaffNameByID(ctx, *attendance.CheckedOutBy)
	}
}

// getStaffNameByID retrieves staff member's full name by ID
func (s *service) getStaffNameByID(ctx context.Context, staffID int64) string {
	staff, err := s.StaffRepo.FindByID(ctx, staffID)
	if err != nil || staff == nil {
		return ""
	}

	person, err := s.UsersService.Get(ctx, staff.PersonID)
	if err != nil || person == nil {
		return ""
	}

	return fmt.Sprintf("%s %s", person.FirstName, person.LastName)
}

// ToggleStudentAttendance toggles the attendance state (check-in or check-out)
// skipAuthCheck: if true, skips authorization check (used when caller already authorized).
//
// IMPORTANT: only safe when the caller serializes scans (e.g. an IoT kiosk).
// Web callers MUST use CheckInStudent / CheckOutStudent instead — under
// concurrency the read-then-flip here can swap a desired "in" into an "out"
// because the second caller's internal re-read sees the first caller's
// commit and flips the action.
func (s *service) ToggleStudentAttendance(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*AttendanceResult, error) {
	authorizedStaffID, err := s.authorizeAttendanceToggle(ctx, studentID, staffID, deviceID, skipAuthCheck)
	if err != nil {
		return nil, err
	}

	currentStatus, err := s.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: err}
	}

	now := time.Now()
	today := timezone.TodayDate()

	// "on_yard" is a sub-state of "checked_in" (still on premises) — toggling
	// from either should perform a checkout. Only "not_checked_in" and
	// "checked_out" start a fresh check-in.
	if currentStatus.Status == "not_checked_in" || currentStatus.Status == "checked_out" {
		return s.performCheckIn(ctx, studentID, authorizedStaffID, deviceID, now, today)
	}

	return s.performCheckOut(ctx, studentID, authorizedStaffID, now, checkoutTypeToggle)
}

// CheckInStudent applies "in" unconditionally. Concurrency-safe: the
// underlying insert uses ON CONFLICT DO NOTHING against the partial unique
// index, so the loser of an in/in race is absorbed and reports the existing
// open row as the canonical result. Action is always "checked_in".
func (s *service) CheckInStudent(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*AttendanceResult, error) {
	authorizedStaffID, err := s.authorizeAttendanceToggle(ctx, studentID, staffID, deviceID, skipAuthCheck)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	today := timezone.TodayDate()
	return s.performCheckIn(ctx, studentID, authorizedStaffID, deviceID, now, today)
}

// CheckOutStudent applies "out" unconditionally via the state-checked
// CloseOpenForToday repository method. If no open row exists (already closed,
// never checked in, or another concurrent caller already closed it), returns
// an idempotent successful result with Action="checked_out" and no
// AttendanceID — the caller can re-fetch the latest status if they need
// timestamps for display. Any open room visit is ended as part of the same
// operation (issue #895) — callers don't need a separate EndVisit call.
func (s *service) CheckOutStudent(ctx context.Context, studentID, staffID int64, skipAuthCheck bool) (*AttendanceResult, error) {
	// Auth path is shared with the toggle — pass deviceID=0 because the
	// caller is web-side (no kiosk involved); IsIoTDeviceRequest is false
	// for web so authorizeWebToggle runs and validates teacher access.
	authorizedStaffID, err := s.authorizeAttendanceToggle(ctx, studentID, staffID, 0, skipAuthCheck)
	if err != nil {
		return nil, err
	}
	return s.performCheckOut(ctx, studentID, authorizedStaffID, time.Now(), checkoutTypeWeb)
}

// CheckOutStudentFromDevice applies "out" for an IoT device after resolving
// the device's active session supervisor as the auditable checkout principal.
func (s *service) CheckOutStudentFromDevice(ctx context.Context, studentID, deviceID int64) (*AttendanceResult, error) {
	authorizedStaffID, err := s.authorizeIoTDeviceToggle(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	return s.performCheckOut(ctx, studentID, authorizedStaffID, time.Now(), checkoutTypeDaily)
}

// authorizeAttendanceToggle handles authorization and returns the staff ID to use
func (s *service) authorizeAttendanceToggle(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (int64, error) {
	if skipAuthCheck {
		return staffID, nil
	}

	isIoTDevice := device.IsIoTDeviceRequest(ctx)

	if isIoTDevice {
		return s.authorizeIoTDeviceToggle(ctx, deviceID)
	}

	return s.authorizeWebToggle(ctx, studentID, staffID)
}

// authorizeWebToggle authorizes web/manual attendance toggle
func (s *service) authorizeWebToggle(ctx context.Context, studentID, staffID int64) (int64, error) {
	isAuthorized, err := s.checkTeacherOrRoomSupervisorAccess(ctx, studentID, staffID)
	if err != nil {
		return 0, err
	}

	if !isAuthorized {
		return 0, &ActiveError{
			Op:  "ToggleStudentAttendance",
			Err: fmt.Errorf("teacher does not have access to this student (not their educational group teacher or room supervisor)"),
		}
	}

	return staffID, nil
}

// authorizeIoTDeviceToggle authorizes IoT device attendance toggle
func (s *service) authorizeIoTDeviceToggle(ctx context.Context, deviceID int64) (int64, error) {
	supervisorStaffID, err := s.getDeviceSupervisorID(ctx, deviceID)
	if err != nil {
		return 0, &ActiveError{
			Op:  "ToggleStudentAttendance",
			Err: fmt.Errorf("device must have an active group with supervisors: %w", err),
		}
	}
	return supervisorStaffID, nil
}

// checkTeacherOrRoomSupervisorAccess checks if teacher has access via educational groups or room supervision
func (s *service) checkTeacherOrRoomSupervisorAccess(ctx context.Context, studentID, staffID int64) (bool, error) {
	// First check via educational groups
	hasAccess, err := s.CheckTeacherStudentAccess(ctx, staffID, studentID)
	if err == nil && hasAccess {
		return true, nil
	}

	// Check via room supervision
	return s.checkRoomSupervisorAccess(ctx, studentID, staffID)
}

// checkRoomSupervisorAccess checks if staff is supervising the student's current room
func (s *service) checkRoomSupervisorAccess(ctx context.Context, studentID, staffID int64) (bool, error) {
	currentVisit, err := s.GetStudentCurrentVisit(ctx, studentID)
	if err != nil || currentVisit == nil || currentVisit.ActiveGroupID == 0 {
		return false, nil
	}

	activeGroup, err := s.GetActiveGroup(ctx, currentVisit.ActiveGroupID)
	if err != nil || activeGroup == nil || !activeGroup.IsActive() {
		return false, nil
	}

	supervisors, err := s.FindSupervisorsByActiveGroupID(ctx, activeGroup.ID)
	if err != nil {
		return false, nil
	}

	for _, supervisor := range supervisors {
		if supervisor.StaffID == staffID && supervisor.EndDate == nil {
			return true, nil
		}
	}

	return false, nil
}

// performCheckIn creates a new attendance record for check-in.
// deviceID == 0 marks a web-originated check-in with no kiosk involved;
// in that case we fall back to the virtual WEB-MANUAL-001 device so the
// FK column always points at a real iot.devices row.
//
// The insert is guarded by a partial unique index on
// (student_id, date) WHERE check_out_time IS NULL (migration 1.15.42), so a
// concurrent second "in" call is silently absorbed via ON CONFLICT and we
// re-fetch the open row to return as the canonical result.
func (s *service) performCheckIn(ctx context.Context, studentID, staffID, deviceID int64, now time.Time, today timezone.Date) (*AttendanceResult, error) {
	// Reject (and lock against a concurrent graduation) a graduated alumnus
	// before writing attendance — the binary-mode / attendance-toggle counterpart
	// to the CreateVisit guard (#405).
	if err := s.ensureStudentCheckinAllowed(ctx, studentID); err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: err}
	}

	resolvedDeviceID := s.resolveDeviceIDForAttendance(ctx, deviceID)
	attendance := &active.Attendance{
		StudentID:   studentID,
		Date:        today,
		CheckInTime: now,
		CheckedInBy: staffID,
		DeviceID:    resolvedDeviceID,
	}
	attendance.SetTenantID(tenant.FromContext(ctx))

	inserted, err := s.AttendanceRepo.CreateIfNoOpenForToday(ctx, attendance)
	if err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: err}
	}
	if !inserted {
		// Another concurrent "in" already created the open attendance row.
		// Treat as success — the desired end state (open attendance) holds.
		existing, fetchErr := s.AttendanceRepo.GetStudentCurrentStatus(ctx, studentID)
		if fetchErr != nil {
			return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fetchErr}
		}
		s.autoClearStudentSickness(ctx, studentID)
		s.autoClearStudentExcused(ctx, studentID)
		s.autoClearPlannedStudentStatuses(ctx, studentID)
		if s.GetPresenceMode(ctx) == "binary" && s.AttendanceSyncer != nil {
			s.AttendanceSyncer.MirrorCheckInAt(ctx, studentID, existing.CheckInTime)
		}
		return &AttendanceResult{
			Action:       "checked_in",
			AttendanceID: existing.ID,
			StudentID:    studentID,
			Timestamp:    existing.CheckInTime,
		}, nil
	}

	s.autoClearStudentSickness(ctx, studentID)
	s.autoClearStudentExcused(ctx, studentID)
	s.autoClearPlannedStudentStatuses(ctx, studentID)
	if s.GetPresenceMode(ctx) == "binary" && s.AttendanceSyncer != nil {
		s.AttendanceSyncer.MirrorCheckInAt(ctx, studentID, now)
	}

	s.trackProductEvent(ctx, "student_checked_in", map[string]any{
		"method": attendanceMethod(ctx),
	})

	return &AttendanceResult{
		Action:       "checked_in",
		AttendanceID: attendance.ID,
		StudentID:    studentID,
		Timestamp:    now,
	}, nil
}

// endOpenVisitForStudent enforces the invariant "attendance checked_out =>
// no open visit" (issue #895). It returns the ended row so callers can mirror
// the same checkout into slot attendance. A missing visit returns nil; every
// other failure propagates so the request transaction rolls back.
func (s *service) endOpenVisitForStudent(ctx context.Context, studentID int64) (*active.Visit, error) {
	visit, err := s.GetStudentCurrentVisit(ctx, studentID)
	if err != nil {
		if errors.Is(err, ErrVisitNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if err := s.VisitRepo.EndVisit(ctx, visit.ID); err != nil {
		latest, findErr := s.VisitRepo.FindByID(ctx, visit.ID)
		if findErr == nil && latest != nil && latest.ExitTime != nil {
			return latest, nil
		}
		return nil, err
	}
	ended, err := s.VisitRepo.FindByID(ctx, visit.ID)
	if err != nil || ended == nil || ended.ExitTime == nil {
		if err != nil {
			return nil, err
		}
		return nil, ErrVisitNotFound
	}
	return ended, nil
}

// performCheckOut closes the open attendance row for the student via a
// state-checked UPDATE WHERE check_out_time IS NULL. Three key properties:
//
//  1. Concurrency-safe: a second concurrent "out" call (or an "in" call that
//     lost the race against another "out") simply finds no open row to
//     update; we report idempotent success rather than corrupting state.
//  2. Yard sub-state is cleared as part of the same UPDATE so detailed-mode
//     callers don't observe an inconsistent (CheckOutTime set, YardSince
//     still set) row even briefly.
//  3. Any open room visit is ended in the same request transaction (issue
//     #895) — including on the idempotent no-open-row path, so a checkout of
//     any kind heals an orphaned visit left behind by older code.
func (s *service) performCheckOut(ctx context.Context, studentID, staffID int64, now time.Time, checkoutType string) (*AttendanceResult, error) {
	closed, err := s.AttendanceRepo.CloseOpenForToday(ctx, studentID, now, staffID)
	if err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fmt.Errorf("database error during state-checked checkout: %w", err)}
	}

	endedVisit, err := s.endOpenVisitForStudent(ctx, studentID)
	if err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fmt.Errorf("end open visit during checkout: %w", err)}
	}

	if s.AttendanceSyncer != nil {
		if s.GetPresenceMode(ctx) == "binary" {
			// Binary mode has no visit provenance, so close the latest mirrored
			// open slot. Run this even for idempotent attendance checkout to heal
			// slot rows left open by older code.
			s.AttendanceSyncer.MirrorCheckOutAt(ctx, studentID, now)
		} else if endedVisit != nil {
			// Detailed mode has exact source provenance through the ended visit.
			s.AttendanceSyncer.MirrorCheckOutForVisit(ctx, endedVisit)
		}
	}

	if closed == nil {
		// No open row — student is already checked out (or never checked in
		// today). Report idempotent success so the caller treats this the
		// same as an actual close.
		return &AttendanceResult{
			Action:    "checked_out",
			StudentID: studentID,
			Timestamp: now,
		}, nil
	}
	s.trackProductEvent(ctx, "student_checked_out", map[string]any{
		"method":        attendanceMethod(ctx),
		"checkout_type": checkoutType,
	})

	return &AttendanceResult{
		Action:       "checked_out",
		AttendanceID: closed.ID,
		StudentID:    studentID,
		Timestamp:    now,
	}, nil
}

// getDeviceSupervisorID retrieves the supervisor staff ID for a device's active group
func (s *service) getDeviceSupervisorID(ctx context.Context, deviceID int64) (int64, error) {
	// Find active group for device
	activeGroup, err := s.GroupRepo.FindActiveByDeviceID(ctx, deviceID)
	if err != nil {
		// Handle case where no active group exists for this device
		if errors.Is(err, ErrNoActiveSession) {
			return 0, fmt.Errorf("no active group assigned to device %d", deviceID)
		}
		return 0, fmt.Errorf("error finding active group for device %d: %w", deviceID, err)
	}

	if activeGroup == nil {
		return 0, fmt.Errorf("no active group assigned to device %d", deviceID)
	}

	// Get supervisors for the active group
	supervisors, err := s.FindSupervisorsByActiveGroupID(ctx, activeGroup.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to get supervisors for group %d: %w", activeGroup.ID, err)
	}

	if len(supervisors) == 0 {
		return 0, fmt.Errorf("no supervisors assigned to active group %d", activeGroup.ID)
	}

	// Use first active supervisor
	now := time.Now()
	for _, supervisor := range supervisors {
		if IsSupervisorActive(supervisor, now) {
			return supervisor.StaffID, nil
		}
	}

	return 0, fmt.Errorf("no active supervisors found in group %d", activeGroup.ID)
}

// CheckTeacherStudentAccess checks if a teacher has access to mark attendance for a student
func (s *service) CheckTeacherStudentAccess(ctx context.Context, teacherID, studentID int64) (bool, error) {
	// Get teacher from staff ID
	teacher, err := s.TeacherRepo.FindByStaffID(ctx, teacherID)
	if err != nil {
		return false, &ActiveError{Op: "CheckTeacherStudentAccess", Err: err}
	}
	if teacher == nil {
		return false, nil
	}

	// Get teacher's groups via educationService
	teacherGroups, err := s.EducationService.GetTeacherGroups(ctx, teacher.ID)
	if err != nil {
		return false, &ActiveError{Op: "CheckTeacherStudentAccess", Err: err}
	}

	// Get student info
	student, err := s.StudentRepo.FindByID(ctx, studentID)
	if err != nil {
		if base.IsNoRows(err) {
			return false, nil
		}
		return false, &ActiveError{Op: "CheckTeacherStudentAccess", Err: err}
	}
	if student == nil || student.GroupID == nil {
		return false, nil
	}

	// Check if student.GroupID is in teacher's groups
	for _, group := range teacherGroups {
		if group.ID == *student.GroupID {
			return true, nil
		}
	}

	return false, nil
}

// BroadcastDailyCheckout sends an SSE student_checkout event to the student's
// educational (OGS) group topic so the "Meine Gruppe" page updates in real time.
// Called after the daily checkout attendance toggle succeeds.
func (s *service) BroadcastDailyCheckout(ctx context.Context, studentID int64) {
	if s.Broadcaster == nil {
		return
	}

	studentIDStr := fmt.Sprintf("%d", studentID)
	studentName, studentRec := s.getStudentDisplayData(ctx, studentID)

	source := "daily_checkout"
	event := realtime.NewEvent(
		realtime.EventStudentCheckOut,
		"", // no active group — student already left their room
		realtime.EventData{
			StudentID:   &studentIDStr,
			StudentName: &studentName,
			Source:      &source,
		},
	)

	// Broadcast to educational (OGS) group topic so the "Meine Gruppe" page updates
	s.broadcastToEducationalGroup(ctx, studentRec, event)

	// Notify all clients so dashboard counts and search page refresh —
	// the educational group broadcast only reaches staff in that group,
	// but the search page is used by all staff.
	_ = s.Broadcaster.BroadcastToAll(realtime.NewEvent(realtime.EventDashboardCountsChanged, "", realtime.EventData{}))
}

// ConfirmDailyCheckout processes the deferred daily-checkout confirmation for an
// IoT device. Normally the student's visit was already ended by the checkin
// handler (student is "unterwegs") and this only updates the attendance record
// when the student confirms "nach Hause". If a visit is still open,
// CheckOutStudentFromDevice ends it in the same request transaction (issue #895).
//
// The student must already have an attendance record for today (status
// "checked_in" or "checked_out"); otherwise ErrNoAttendanceRecordForCheckout is
// returned. Attendance is only mutated when destination is "zuhause" and the
// student is still "checked_in"; a concurrent checkout is treated as an
// idempotent no-op.
func (s *service) ConfirmDailyCheckout(ctx context.Context, studentID, deviceID int64, destination string) (*DailyCheckoutResult, error) {
	s.getLogger().InfoContext(ctx, "confirming daily checkout",
		slog.Int64("student_id", studentID),
		slog.String("destination", destination),
	)

	currentStatus, err := s.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		s.getLogger().ErrorContext(ctx, "failed to get attendance status",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if currentStatus.Status != "checked_in" && currentStatus.Status != "checked_out" {
		s.getLogger().ErrorContext(ctx, "student has no attendance record for today",
			slog.Int64("student_id", studentID),
			slog.String("status", currentStatus.Status),
		)
		return nil, ErrNoAttendanceRecordForCheckout
	}

	if destination == "zuhause" {
		switch currentStatus.Status {
		case "checked_out":
			s.getLogger().DebugContext(ctx, "student already checked out, skipping attendance toggle",
				slog.Int64("student_id", studentID),
			)
		case "checked_in":
			if _, err := s.CheckOutStudentFromDevice(ctx, studentID, deviceID); err != nil {
				s.getLogger().ErrorContext(ctx, "failed to update attendance for daily checkout",
					slog.Int64("student_id", studentID),
					slog.String("error", err.Error()),
				)
				return nil, err
			}

			// Broadcast SSE event so the OGS Groups page updates in real time
			s.BroadcastDailyCheckout(ctx, studentID)
		}
	}

	action := "checked_out_daily"
	if destination == "unterwegs" {
		action = "checked_out"
	}

	s.getLogger().InfoContext(ctx, "daily checkout confirmed",
		slog.Int64("student_id", studentID),
		slog.String("action", action),
		slog.String("destination", destination),
	)

	return &DailyCheckoutResult{Action: action}, nil
}

// ======== Unclaimed Groups Management (Deviceless Claiming) ========

// GetUnclaimedActiveGroups returns all active groups that have no supervisors
// This is used for deviceless rooms like Schulhof where teachers claim supervision via frontend
func (s *service) GetUnclaimedActiveGroups(ctx context.Context) ([]*active.Group, error) {
	groups, err := s.GroupRepo.FindUnclaimed(ctx)
	if err != nil {
		return nil, &ActiveError{Op: "GetUnclaimedActiveGroups", Err: err}
	}

	return groups, nil
}

// ClaimActiveGroup allows a staff member to claim supervision of an active group
// This is primarily used for deviceless rooms like Schulhof
func (s *service) ClaimActiveGroup(ctx context.Context, groupID, staffID int64, role string) (*active.GroupSupervisor, error) {
	// Verify group exists and is still active
	group, err := s.GroupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &ActiveError{Op: "ClaimActiveGroup", Err: errors.New("active group not found")}
	}

	if group.EndTime != nil {
		return nil, &ActiveError{Op: "ClaimActiveGroup", Err: errors.New("cannot claim ended group")}
	}

	// Check if staff is already supervising this group (only check active supervisors)
	existingSupervisors, err := s.SupervisorRepo.FindByActiveGroupID(ctx, groupID, true)
	if err == nil {
		for _, sup := range existingSupervisors {
			if sup.StaffID == staffID {
				return nil, &ActiveError{Op: "ClaimActiveGroup", Err: ErrStaffAlreadySupervising}
			}
		}
	}

	// Create supervisor assignment
	if role == "" {
		role = "supervisor"
	}

	supervisor := &active.GroupSupervisor{
		StaffID:   staffID,
		GroupID:   groupID,
		Role:      role,
		StartDate: timezone.TodayDate(),
		// EndDate is nil (active supervision)
	}

	// Use existing CreateGroupSupervisor method for validation and creation
	if err := s.CreateGroupSupervisor(ctx, supervisor); err != nil {
		return nil, err
	}

	return supervisor, nil
}
