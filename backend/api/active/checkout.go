package active

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// checkoutStudent handles immediate checkout of a student
func (rs *Resource) checkoutStudent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user from JWT context
	userClaims := jwt.ClaimsFromCtx(ctx)
	if userClaims.ID == 0 {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Get student ID from URL
	studentIDStr := chi.URLParam(r, "studentId")
	studentID, err := strconv.ParseInt(studentIDStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid student ID")
		return
	}

	// First check if student has a current visit (in a room)
	currentVisit, _ := rs.ActiveService.GetStudentCurrentVisit(ctx, studentID)

	// Check attendance status regardless of visit
	attendanceStatus, err := rs.ActiveService.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get attendance status")
		return
	}

	// If student is not checked in at all, return error
	if attendanceStatus.Status != "checked_in" {
		common.RespondWithError(w, r, http.StatusNotFound, "Student is not currently checked in")
		return
	}

	// Check authorization - only education group teachers can checkout their students
	// IMPORTANT: Validate staff BEFORE ending visit to prevent data inconsistency

	// Get the person and staff info for the current user
	person, err := rs.PersonService.FindByAccountID(ctx, int64(userClaims.ID))
	if err != nil || person == nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get user information")
		return
	}

	staff, err := rs.PersonService.StaffRepository().FindByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "User is not a staff member")
		return
	}

	// Check if user is a teacher of the student's education group
	hasAccess, err := rs.ActiveService.CheckTeacherStudentAccess(ctx, staff.ID, studentID)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to check authorization")
		return
	}
	if !hasAccess {
		common.RespondWithError(w, r, http.StatusForbidden,
			"You are not authorized to checkout this student")
		return
	}

	// Now that we've validated staff exists and is authorized, end the visit
	if currentVisit != nil {
		if err := rs.ActiveService.EndVisit(ctx, currentVisit.ID); err != nil {
			// Log but don't fail - we still want to update attendance
			fmt.Printf("Warning: Failed to end visit %d: %v\n", currentVisit.ID, err)
		}
	}

	// Toggle attendance to check out the student
	// Note: deviceID = 0 for web-based checkouts (no physical device)
	result, err := rs.ActiveService.ToggleStudentAttendance(ctx, studentID, staff.ID, 0)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to checkout student from daily attendance")
		return
	}

	// Cancel any pending scheduled checkout since the student was checked out manually
	pendingCheckout, err := rs.ActiveService.GetPendingScheduledCheckout(ctx, studentID)
	if err != nil {
		// Log warning but don't fail the checkout
		fmt.Printf("Warning: Failed to check for pending scheduled checkout: %v\n", err)
	} else if pendingCheckout != nil {
		// Cancel using the staff ID who performed the manual checkout
		if err := rs.ActiveService.CancelScheduledCheckout(ctx, pendingCheckout.ID, staff.ID); err != nil {
			fmt.Printf("Warning: Failed to cancel scheduled checkout %d: %v\n", pendingCheckout.ID, err)
		} else {
			fmt.Printf("Cancelled pending scheduled checkout %d for student %d\n", pendingCheckout.ID, studentID)
		}
	}

	common.RespondWithJSON(w, r, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Student checked out successfully",
		"data": map[string]interface{}{
			"student_id":    studentID,
			"action":        result.Action,
			"attendance_id": result.AttendanceID,
		},
	})
}
