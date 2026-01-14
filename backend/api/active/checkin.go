package active

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/logging"
)

// CheckinRequest represents the request body for manual check-in
type CheckinRequest struct {
	ActiveGroupID int64 `json:"active_group_id"`
}

// checkinStudent handles manual check-in of a student who is at home
// The request body must contain an active_group_id to specify which room to check into
func (rs *Resource) checkinStudent(w http.ResponseWriter, r *http.Request) {
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

	// Parse request body
	var req CheckinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate active_group_id is provided
	if req.ActiveGroupID <= 0 {
		common.RespondWithError(w, r, http.StatusBadRequest, "active_group_id is required")
		return
	}

	// Get and validate the active group
	activeGroup, err := rs.ActiveService.GetActiveGroup(ctx, req.ActiveGroupID)
	if err != nil || activeGroup == nil {
		common.RespondWithError(w, r, http.StatusNotFound, "Active group not found")
		return
	}

	// Verify the group is still active
	if !activeGroup.IsActive() {
		common.RespondWithError(w, r, http.StatusConflict, "The selected room session is no longer active")
		return
	}

	// Check attendance status
	attendanceStatus, err := rs.ActiveService.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get attendance status")
		return
	}

	// Student must be at home (checked_out or not_checked_in) to check in
	if attendanceStatus.Status == "checked_in" {
		common.RespondWithError(w, r, http.StatusConflict, "Student is already checked in")
		return
	}

	// Check authorization - get staff info
	person, err := rs.PersonService.FindByAccountID(ctx, int64(userClaims.ID))
	if err != nil || person == nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get user information")
		return
	}

	staff, err := rs.PersonService.StaffRepository().FindByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		common.RespondWithError(w, r, http.StatusForbidden, "Only staff members can check in students")
		return
	}

	// Check if teacher has access to this student (is assigned to their group)
	hasAccess, err := rs.ActiveService.CheckTeacherStudentAccess(ctx, staff.ID, studentID)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to check access permissions")
		return
	}

	if !hasAccess {
		common.RespondWithError(w, r, http.StatusForbidden,
			"You are not authorized to check in this student. You must be their group teacher.")
		return
	}

	// Create visit for the selected active group
	visit := &activeModels.Visit{
		StudentID:     studentID,
		ActiveGroupID: req.ActiveGroupID,
		EntryTime:     time.Now(),
	}

	if err := rs.ActiveService.CreateVisit(ctx, visit); err != nil {
		if logging.Logger != nil {
			logging.Logger.WithFields(map[string]interface{}{
				"student_id":      studentID,
				"active_group_id": req.ActiveGroupID,
				"error":           err.Error(),
			}).Error("Failed to create visit during check-in")
		}
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to check in student to room")
		return
	}

	// Get updated attendance status for response
	updatedAttendance, statusErr := rs.ActiveService.GetStudentAttendanceStatus(ctx, studentID)
	if statusErr != nil {
		if logging.Logger != nil {
			logging.Logger.WithFields(map[string]interface{}{
				"student_id": studentID,
				"error":      statusErr.Error(),
			}).Warn("Failed to get updated attendance status after checkin")
		}
	}

	responseData := map[string]interface{}{
		"student_id":      studentID,
		"action":          "checked_in",
		"visit_id":        visit.ID,
		"active_group_id": req.ActiveGroupID,
		"room_id":         activeGroup.RoomID,
	}

	if statusErr == nil && updatedAttendance != nil {
		responseData["attendance_status"] = updatedAttendance.Status
		responseData["check_in_time"] = updatedAttendance.CheckInTime
		responseData["checked_in_by"] = updatedAttendance.CheckedInBy
	}

	common.RespondWithJSON(w, r, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Student checked in successfully",
		"data":    responseData,
	})
}
