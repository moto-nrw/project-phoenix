package active

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
)

// checkoutContext holds all context needed for a checkout operation
type checkoutContext struct {
	StudentID        int64
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

// getCheckoutContext retrieves the attendance status for a student
func (rs *Resource) getCheckoutContext(ctx context.Context, studentID int64) (*checkoutContext, error) {
	// Get attendance status (required)
	attendanceStatus, err := rs.ActiveService.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attendance status: %w", err)
	}

	// Validate student is checked in
	if attendanceStatus.Status != "checked_in" {
		return nil, ErrNotCheckedIn
	}

	return &checkoutContext{
		StudentID:        studentID,
		AttendanceStatus: attendanceStatus,
	}, nil
}

// authorizeStudentCheckout verifies the user can checkout this student
// Returns the staff record if authorized, error otherwise
// Note: Any authenticated staff member can checkout any checked-in student
func (rs *Resource) authorizeStudentCheckout(ctx context.Context) (*users.Staff, error) {
	staff, err := rs.UserContextService.GetCurrentStaff(ctx)
	if err != nil {
		if errors.Is(err, usercontext.ErrUserNotLinkedToPerson) ||
			errors.Is(err, usercontext.ErrUserNotLinkedToStaff) {
			return nil, ErrNotAuthorized
		}
		return nil, ErrStaffNotFound
	}
	if staff == nil {
		return nil, ErrNotAuthorized
	}

	return staff, nil
}

// executeStudentCheckout performs the actual checkout operation
func (rs *Resource) executeStudentCheckout(
	ctx context.Context,
	staff *users.Staff,
	checkoutCtx *checkoutContext,
) (*checkoutResult, error) {
	// Embed staff in context for visit-end recording
	actionCtx := context.WithValue(ctx, device.CtxStaff, staff)

	// Action-explicit, race-safe checkout (issue #895). The service closes
	// the attendance row AND ends any open visit in the same request
	// transaction; any failure propagates so the handler responds 500 and
	// TenantTxMiddleware rolls the whole request back — never a checked-out
	// attendance row alongside an orphaned open visit.
	result, err := rs.ActiveService.CheckOutStudent(actionCtx, checkoutCtx.StudentID, staff.ID, true)
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

// getUpdatedAttendanceStatus fetches the updated attendance status (optional)
func (rs *Resource) getUpdatedAttendanceStatus(ctx context.Context, studentID int64) *activeService.AttendanceStatus {
	status, err := rs.ActiveService.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		rs.getLogger().WarnContext(ctx, "failed to get updated attendance status after checkout",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
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
