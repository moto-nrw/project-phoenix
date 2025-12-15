package active

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/users"
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

	// Check authorization - teachers supervising the student's current room can checkout
	isAuthorized := false

	// Get the person and staff info for the current user (declare at function scope for later use)
	var person *users.Person
	var staff *users.Staff

	person, err = rs.PersonService.FindByAccountID(ctx, int64(userClaims.ID))
	if err == nil && person != nil {
		staff, err = rs.PersonService.StaffRepository().FindByPersonID(ctx, person.ID)
		if err == nil && staff != nil {
			// If student has a current visit, check if teacher is supervising that room
			if currentVisit != nil && currentVisit.ActiveGroupID > 0 {
				// Get the active group to find supervisors
				activeGroup, err := rs.ActiveService.GetActiveGroup(ctx, currentVisit.ActiveGroupID)
				if err == nil && activeGroup != nil && activeGroup.IsActive() {
					// Check if this staff member is supervising this active group
					supervisors, err := rs.ActiveService.FindSupervisorsByActiveGroupID(ctx, activeGroup.ID)
					if err == nil {
						for _, supervisor := range supervisors {
							if supervisor.StaffID == staff.ID && supervisor.EndDate == nil {
								isAuthorized = true
								break
							}
						}
					}
				}
			}

			// Fallback: Also allow teachers assigned to student's educational group
			if !isAuthorized {
				hasAccess, err := rs.ActiveService.CheckTeacherStudentAccess(ctx, staff.ID, studentID)
				if err == nil && hasAccess {
					isAuthorized = true
				}
			}
		}
	}

	if !isAuthorized {
		common.RespondWithError(w, r, http.StatusForbidden,
			"You are not authorized to checkout this student. You must be supervising their current room or be their group teacher.")
		return
	}

	// Ensure we have staff info for follow-up actions (e.g., cancelling scheduled checkouts)
	if person == nil || staff == nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get staff information")
		return
	}

	// Embed staff in context so EndVisit can record who performed the checkout
	actionCtx := context.WithValue(ctx, device.CtxStaff, staff)

	// End the visit if student has one
	if currentVisit != nil {
		if err := rs.ActiveService.EndVisit(actionCtx, currentVisit.ID); err != nil {
			// Log but don't fail - we still want to update attendance
			fmt.Printf("Warning: Failed to end visit %d: %v\n", currentVisit.ID, err)
		}
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

	// Use original ctx (not actionCtx) to avoid transaction conflicts from EndVisit
	// Pass skipAuthCheck=true because we already authorized above (before ending the visit)
	result, err := rs.ActiveService.ToggleStudentAttendance(ctx, studentID, staff.ID, 0, true)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to checkout student from daily attendance")
		return
	}

	updatedAttendance, statusErr := rs.ActiveService.GetStudentAttendanceStatus(ctx, studentID)
	if statusErr != nil {
		// Continue even if we can't get updated status
	}

	responseData := map[string]interface{}{
		"student_id":    studentID,
		"action":        result.Action,
		"attendance_id": result.AttendanceID,
	}

	if statusErr == nil && updatedAttendance != nil {
		responseData["attendance_status"] = updatedAttendance.Status
		responseData["check_in_time"] = updatedAttendance.CheckInTime
		responseData["check_out_time"] = updatedAttendance.CheckOutTime
		responseData["checked_in_by"] = updatedAttendance.CheckedInBy
		responseData["checked_out_by"] = updatedAttendance.CheckedOutBy
	}

	common.RespondWithJSON(w, r, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Student checked out successfully",
		"data":    responseData,
	})
}
