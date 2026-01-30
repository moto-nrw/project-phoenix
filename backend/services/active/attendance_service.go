package active

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
)

// Attendance tracking operations

func (s *service) GetStudentsAttendanceStatuses(ctx context.Context, studentIDs []int64) (map[int64]*AttendanceStatus, error) {
	if len(studentIDs) == 0 {
		return map[int64]*AttendanceStatus{}, nil
	}

	statuses := make(map[int64]*AttendanceStatus, len(studentIDs))

	attendanceRecords, err := s.attendanceRepo.GetTodayByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetStudentsAttendanceStatuses", Err: ErrDatabaseOperation}
	}
	if attendanceRecords == nil {
		attendanceRecords = make(map[int64]*active.Attendance)
	}

	// Use timezone.Today() for consistent Europe/Berlin timezone handling
	today := timezone.Today()

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
			if attendance.CheckOutTime != nil {
				status.Status = "checked_out"
			} else {
				status.Status = "checked_in"
			}
		}

		statuses[studentID] = status
	}

	return statuses, nil
}

// GetStudentAttendanceStatus gets today's latest attendance record and determines status
func (s *service) GetStudentAttendanceStatus(ctx context.Context, studentID int64) (*AttendanceStatus, error) {
	attendance, err := s.attendanceRepo.GetStudentCurrentStatus(ctx, studentID)
	if err != nil {
		// Use timezone.Today() for consistent Europe/Berlin timezone handling
		return &AttendanceStatus{
			StudentID: studentID,
			Status:    "not_checked_in",
			Date:      timezone.Today(),
		}, nil
	}

	status := "checked_in"
	if attendance.CheckOutTime != nil {
		status = "checked_out"
	}

	result := &AttendanceStatus{
		StudentID:    studentID,
		Status:       status,
		Date:         attendance.Date,
		CheckInTime:  &attendance.CheckInTime,
		CheckOutTime: attendance.CheckOutTime,
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
	staff, err := s.staffRepo.FindByID(ctx, staffID)
	if err != nil || staff == nil {
		return ""
	}

	person, err := s.usersService.Get(ctx, staff.PersonID)
	if err != nil || person == nil {
		return ""
	}

	return fmt.Sprintf("%s %s", person.FirstName, person.LastName)
}

// ToggleStudentAttendance toggles the attendance state (check-in or check-out)
// skipAuthCheck: if true, skips authorization check (used when caller already authorized)
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
	// Use timezone.Today() for consistent Europe/Berlin timezone handling
	today := timezone.Today()

	if currentStatus.Status == "not_checked_in" || currentStatus.Status == "checked_out" {
		return s.performCheckIn(ctx, studentID, authorizedStaffID, deviceID, now, today)
	}

	return s.performCheckOut(ctx, studentID, authorizedStaffID, now)
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

// performCheckIn creates a new attendance record for check-in
func (s *service) performCheckIn(ctx context.Context, studentID, staffID, deviceID int64, now, today time.Time) (*AttendanceResult, error) {
	attendance := &active.Attendance{
		StudentID:   studentID,
		Date:        today,
		CheckInTime: now,
		CheckedInBy: staffID,
		DeviceID:    deviceID,
	}

	if err := s.attendanceRepo.Create(ctx, attendance); err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: err}
	}

	return &AttendanceResult{
		Action:       "checked_in",
		AttendanceID: attendance.ID,
		StudentID:    studentID,
		Timestamp:    now,
	}, nil
}

// performCheckOut updates attendance record for check-out
func (s *service) performCheckOut(ctx context.Context, studentID, staffID int64, now time.Time) (*AttendanceResult, error) {
	attendance, err := s.attendanceRepo.GetStudentCurrentStatus(ctx, studentID)
	if err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: err}
	}

	attendance.CheckOutTime = &now
	if staffID > 0 {
		attendance.CheckedOutBy = &staffID
	}

	if err := s.attendanceRepo.Update(ctx, attendance); err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fmt.Errorf("database error during update: %w", err)}
	}

	return &AttendanceResult{
		Action:       "checked_out",
		AttendanceID: attendance.ID,
		StudentID:    studentID,
		Timestamp:    now,
	}, nil
}

// getDeviceSupervisorID retrieves the supervisor staff ID for a device's active group
func (s *service) getDeviceSupervisorID(ctx context.Context, deviceID int64) (int64, error) {
	// Find active group for device
	activeGroup, err := s.groupRepo.FindActiveByDeviceID(ctx, deviceID)
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
	for _, supervisor := range supervisors {
		if supervisor.IsActive() {
			return supervisor.StaffID, nil
		}
	}

	return 0, fmt.Errorf("no active supervisors found in group %d", activeGroup.ID)
}

// CheckTeacherStudentAccess checks if a teacher has access to mark attendance for a student
func (s *service) CheckTeacherStudentAccess(ctx context.Context, teacherID, studentID int64) (bool, error) {
	// Get teacher from staff ID
	teacher, err := s.teacherRepo.FindByStaffID(ctx, teacherID)
	if err != nil {
		return false, &ActiveError{Op: "CheckTeacherStudentAccess", Err: err}
	}
	if teacher == nil {
		return false, nil
	}

	// Get teacher's groups via educationService
	teacherGroups, err := s.educationService.GetTeacherGroups(ctx, teacher.ID)
	if err != nil {
		return false, &ActiveError{Op: "CheckTeacherStudentAccess", Err: err}
	}

	// Get student info
	student, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil {
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

// ======== Unclaimed Groups Management (Deviceless Claiming) ========

// GetUnclaimedActiveGroups returns all active groups that have no supervisors
// This is used for deviceless rooms like Schulhof where teachers claim supervision via frontend
func (s *service) GetUnclaimedActiveGroups(ctx context.Context) ([]*active.Group, error) {
	groups, err := s.groupRepo.FindUnclaimed(ctx)
	if err != nil {
		return nil, &ActiveError{Op: "GetUnclaimedActiveGroups", Err: err}
	}

	return groups, nil
}

// ClaimActiveGroup allows a staff member to claim supervision of an active group
// This is primarily used for deviceless rooms like Schulhof
func (s *service) ClaimActiveGroup(ctx context.Context, groupID, staffID int64, role string) (*active.GroupSupervisor, error) {
	// Verify group exists and is still active
	group, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &ActiveError{Op: "ClaimActiveGroup", Err: errors.New("active group not found")}
	}

	if group.EndTime != nil {
		return nil, &ActiveError{Op: "ClaimActiveGroup", Err: errors.New("cannot claim ended group")}
	}

	// Check if staff is already supervising this group (only check active supervisors)
	existingSupervisors, err := s.supervisorRepo.FindByActiveGroupID(ctx, groupID, true)
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
		StartDate: time.Now(),
		// EndDate is nil (active supervision)
	}

	// Use existing CreateGroupSupervisor method for validation and creation
	if err := s.CreateGroupSupervisor(ctx, supervisor); err != nil {
		return nil, err
	}

	return supervisor, nil
}
