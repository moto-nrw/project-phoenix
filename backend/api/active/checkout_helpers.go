package active

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// checkoutContext holds all context needed for a checkout operation
type checkoutContext struct {
	StudentID        int64
	CurrentVisit     *active.Visit
	AttendanceStatus *activeService.AttendanceStatus
}

// checkoutResult holds the result of a checkout operation
type checkoutResult struct {
	Result            *activeService.AttendanceResult
	UpdatedAttendance *activeService.AttendanceStatus
}

// Common errors for checkout operations
var (
	ErrNotCheckedIn   = errors.New("student is not currently checked in")
	ErrNotAuthorized  = errors.New("not authorized to checkout this student")
	ErrStaffNotFound  = errors.New("failed to get staff information")
	ErrCheckoutFailed = errors.New("failed to checkout student")
)

// parseStudentIDFromRequest extracts and validates the student ID from URL params
func parseStudentIDFromRequest(r *http.Request) (int64, error) {
	studentIDStr := chi.URLParam(r, "studentId")
	return strconv.ParseInt(studentIDStr, 10, 64)
}

// getCheckoutContext retrieves the current visit and attendance status for a student
func (rs *Resource) getCheckoutContext(ctx context.Context, studentID int64) (*checkoutContext, error) {
	// Get current visit (may be nil if student not in a room)
	currentVisit, _ := rs.ActiveService.GetStudentCurrentVisit(ctx, studentID)

	// Get attendance status (required)
	attendanceStatus, err := rs.ActiveService.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attendance status: %w", err)
	}

	// Validate student is checked in
	if attendanceStatus.Status != activeService.StatusCheckedIn {
		return nil, ErrNotCheckedIn
	}

	return &checkoutContext{
		StudentID:        studentID,
		CurrentVisit:     currentVisit,
		AttendanceStatus: attendanceStatus,
	}, nil
}

// authorizeStudentCheckout verifies the user can checkout this student
// Returns the staff record if authorized, error otherwise
func (rs *Resource) authorizeStudentCheckout(
	ctx context.Context,
	userClaims jwt.AppClaims,
	checkoutCtx *checkoutContext,
) (*users.Staff, error) {
	// Get person from account
	person, err := rs.PersonService.FindByAccountID(ctx, int64(userClaims.ID))
	if err != nil || person == nil {
		return nil, ErrStaffNotFound
	}

	// Get staff record
	staff, err := rs.PersonService.GetStaffByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		return nil, ErrStaffNotFound
	}

	// Check authorization via room supervision first
	if rs.isRoomSupervisor(ctx, staff.ID, checkoutCtx.CurrentVisit) {
		return staff, nil
	}

	// Fallback: check educational group access
	if rs.hasEducationalGroupAccess(ctx, staff.ID, checkoutCtx.StudentID) {
		return staff, nil
	}

	return nil, ErrNotAuthorized
}

// isRoomSupervisor checks if staff is supervising the student's current room
func (rs *Resource) isRoomSupervisor(ctx context.Context, staffID int64, currentVisit *active.Visit) bool {
	if currentVisit == nil || currentVisit.ActiveGroupID <= 0 {
		return false
	}

	activeGroup, err := rs.ActiveService.GetActiveGroup(ctx, currentVisit.ActiveGroupID)
	if err != nil || activeGroup == nil || !activeGroup.IsActive() {
		return false
	}

	supervisors, err := rs.ActiveService.FindSupervisorsByActiveGroupID(ctx, activeGroup.ID)
	if err != nil {
		return false
	}

	for _, supervisor := range supervisors {
		if supervisor.StaffID == staffID && supervisor.EndDate == nil {
			return true
		}
	}

	return false
}

// hasEducationalGroupAccess checks if staff has access via educational group assignment
func (rs *Resource) hasEducationalGroupAccess(ctx context.Context, staffID, studentID int64) bool {
	hasAccess, err := rs.ActiveService.CheckTeacherStudentAccess(ctx, staffID, studentID)
	return err == nil && hasAccess
}

// executeStudentCheckout performs the actual checkout operation
func (rs *Resource) executeStudentCheckout(
	ctx context.Context,
	staff *users.Staff,
	checkoutCtx *checkoutContext,
) (*checkoutResult, error) {
	// Embed staff in context for EndVisit recording
	actionCtx := context.WithValue(ctx, device.CtxStaff, staff)

	// End active visit if exists
	rs.endActiveVisit(actionCtx, checkoutCtx.CurrentVisit)

	// Toggle attendance to checked out
	result, err := rs.ActiveService.ToggleStudentAttendance(
		ctx, checkoutCtx.StudentID, staff.ID, 0, true,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCheckoutFailed, err)
	}

	// Get updated attendance status (optional, don't fail if this errors)
	updatedAttendance := rs.getUpdatedAttendanceStatus(ctx, checkoutCtx.StudentID)

	return &checkoutResult{
		Result:            result,
		UpdatedAttendance: updatedAttendance,
	}, nil
}

// endActiveVisit ends the current visit if one exists (fire-and-forget)
func (rs *Resource) endActiveVisit(ctx context.Context, currentVisit *active.Visit) {
	if currentVisit == nil {
		return
	}

	if err := rs.ActiveService.EndVisit(ctx, currentVisit.ID); err != nil {
		fmt.Printf("Warning: Failed to end visit %d: %v\n", currentVisit.ID, err)
	}
}

// getUpdatedAttendanceStatus fetches the updated attendance status (optional)
func (rs *Resource) getUpdatedAttendanceStatus(ctx context.Context, studentID int64) *activeService.AttendanceStatus {
	status, err := rs.ActiveService.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		if logger.Logger != nil {
			logger.Logger.WithFields(map[string]interface{}{
				"student_id": studentID,
				"error":      err.Error(),
			}).Warn("Failed to get updated attendance status after checkout")
		}
		return nil
	}
	return status
}

// buildCheckoutResponse constructs the JSON response for a successful checkout
func buildCheckoutResponse(studentID int64, result *checkoutResult) map[string]interface{} {
	responseData := map[string]interface{}{
		"student_id":    studentID,
		"action":        result.Result.Action,
		"attendance_id": result.Result.AttendanceID,
	}

	if result.UpdatedAttendance != nil {
		responseData["attendance_status"] = result.UpdatedAttendance.Status
		responseData["check_in_time"] = result.UpdatedAttendance.CheckInTime
		responseData["check_out_time"] = result.UpdatedAttendance.CheckOutTime
		responseData["checked_in_by"] = result.UpdatedAttendance.CheckedInBy
		responseData["checked_out_by"] = result.UpdatedAttendance.CheckedOutBy
	}

	return map[string]interface{}{
		"status":  "success",
		"message": "Student checked out successfully",
		"data":    responseData,
	}
}
