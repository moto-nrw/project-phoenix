package active

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// CheckinRequest represents the request body for manual check-in
type CheckinRequest struct {
	ActiveGroupID int64 `json:"active_group_id"`
}

// checkinContext holds validated data for the check-in operation
type checkinContext struct {
	studentID   int64
	activeGroup *activeModels.Group
	staff       *users.Staff
	request     CheckinRequest
}

// checkinStudent handles manual check-in of a student who is at home
// The request body must contain an active_group_id to specify which room to check into
func (rs *Resource) checkinStudent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse and validate the request
	checkinCtx, err := rs.parseAndValidateCheckinRequest(ctx, r)
	if err != nil {
		err.respond(w, r)
		return
	}

	// Validate student can be checked in
	if err := rs.validateStudentForCheckin(ctx, checkinCtx.studentID); err != nil {
		err.respond(w, r)
		return
	}

	// Create the visit with staff context
	ctx = context.WithValue(ctx, device.CtxStaff, checkinCtx.staff)
	visit, err := rs.createCheckinVisit(ctx, checkinCtx)
	if err != nil {
		err.respond(w, r)
		return
	}

	// Build and send success response
	rs.respondCheckinSuccess(w, r, ctx, visit, checkinCtx)
}

// checkinError represents an error that occurred during check-in
type checkinError struct {
	statusCode int
	message    string
}

func (e *checkinError) respond(w http.ResponseWriter, r *http.Request) {
	common.RespondWithError(w, r, e.statusCode, e.message)
}

// parseAndValidateCheckinRequest parses the request and validates authorization
func (rs *Resource) parseAndValidateCheckinRequest(ctx context.Context, r *http.Request) (*checkinContext, *checkinError) {
	// Get user from JWT context
	userClaims := jwt.ClaimsFromCtx(ctx)
	if userClaims.ID == 0 {
		return nil, &checkinError{http.StatusUnauthorized, "Invalid token"}
	}

	// Get student ID from URL
	studentIDStr := chi.URLParam(r, "studentId")
	studentID, err := strconv.ParseInt(studentIDStr, 10, 64)
	if err != nil {
		return nil, &checkinError{http.StatusBadRequest, "Invalid student ID"}
	}

	// Verify the student exists before any further processing
	student, studentErr := rs.PersonService.StudentRepository().FindByID(ctx, studentID)
	if studentErr != nil || student == nil {
		return nil, &checkinError{http.StatusNotFound, "Student not found"}
	}

	// Parse request body
	var req CheckinRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return nil, &checkinError{http.StatusBadRequest, "Invalid request body"}
	}

	// Validate active_group_id is provided
	if req.ActiveGroupID <= 0 {
		return nil, &checkinError{http.StatusBadRequest, "active_group_id is required"}
	}

	// Get and validate the active group
	activeGroup, groupErr := rs.ActiveService.GetActiveGroup(ctx, req.ActiveGroupID)
	if groupErr != nil {
		// Distinguish "not found" from other errors (DB failures, timeouts)
		if errors.Is(groupErr, activeService.ErrActiveGroupNotFound) {
			return nil, &checkinError{http.StatusNotFound, "Active group not found"}
		}
		return nil, &checkinError{http.StatusInternalServerError, "Failed to retrieve active group"}
	}
	if activeGroup == nil {
		return nil, &checkinError{http.StatusNotFound, "Active group not found"}
	}

	if !activeGroup.IsActive() {
		return nil, &checkinError{http.StatusConflict, "The selected room session is no longer active"}
	}

	// Get staff authorization
	staff, authErr := rs.getAuthorizedStaff(ctx, userClaims.ID, studentID)
	if authErr != nil {
		return nil, authErr
	}

	return &checkinContext{
		studentID:   studentID,
		activeGroup: activeGroup,
		staff:       staff,
		request:     req,
	}, nil
}

// getAuthorizedStaff checks if the user is authorized to check in the student
func (rs *Resource) getAuthorizedStaff(ctx context.Context, accountID int, studentID int64) (*users.Staff, *checkinError) {
	person, personErr := rs.PersonService.FindByAccountID(ctx, int64(accountID))
	if personErr != nil || person == nil {
		return nil, &checkinError{http.StatusInternalServerError, "Failed to get user information"}
	}

	staff, staffErr := rs.PersonService.StaffRepository().FindByPersonID(ctx, person.ID)
	if staffErr != nil || staff == nil {
		return nil, &checkinError{http.StatusForbidden, "Only staff members can check in students"}
	}

	hasAccess, accessErr := rs.ActiveService.CheckTeacherStudentAccess(ctx, staff.ID, studentID)
	if accessErr != nil {
		return nil, &checkinError{http.StatusInternalServerError, "Failed to check access permissions"}
	}

	if !hasAccess {
		return nil, &checkinError{http.StatusForbidden, "You are not authorized to check in this student. You must be their group teacher."}
	}

	return staff, nil
}

// validateStudentForCheckin checks if the student is in a valid state for check-in
func (rs *Resource) validateStudentForCheckin(ctx context.Context, studentID int64) *checkinError {
	// Check if student already has an active visit
	currentVisit, visitErr := rs.ActiveService.GetStudentCurrentVisit(ctx, studentID)
	if visitErr != nil {
		// ErrVisitNotFound is expected when student has no active visit - proceed with check-in
		if !errors.Is(visitErr, activeService.ErrVisitNotFound) {
			// Any other error (DB failure, etc.) should return 500
			return &checkinError{http.StatusInternalServerError, "Failed to check current visit status"}
		}
	}
	if currentVisit != nil {
		return &checkinError{http.StatusConflict, "Student already has an active visit in another room"}
	}

	// Check attendance status
	attendanceStatus, statusErr := rs.ActiveService.GetStudentAttendanceStatus(ctx, studentID)
	if statusErr != nil {
		return &checkinError{http.StatusInternalServerError, "Failed to get attendance status"}
	}

	if attendanceStatus.Status == "checked_in" {
		return &checkinError{http.StatusConflict, "Student is already checked in"}
	}

	return nil
}

// createCheckinVisit creates the visit record for check-in
func (rs *Resource) createCheckinVisit(ctx context.Context, checkinCtx *checkinContext) (*activeModels.Visit, *checkinError) {
	visit := &activeModels.Visit{
		StudentID:     checkinCtx.studentID,
		ActiveGroupID: checkinCtx.request.ActiveGroupID,
		EntryTime:     time.Now(),
	}

	if createErr := rs.ActiveService.CreateVisit(ctx, visit); createErr != nil {
		// Handle race condition: another request already created a visit for this student
		if errors.Is(createErr, activeService.ErrStudentAlreadyActive) {
			return nil, &checkinError{http.StatusConflict, "Student already has an active visit"}
		}
		rs.getLogger().ErrorContext(ctx, "failed to create visit during check-in",
			slog.Int64("student_id", checkinCtx.studentID),
			slog.Int64("active_group_id", checkinCtx.request.ActiveGroupID),
			slog.String("error", createErr.Error()),
		)
		return nil, &checkinError{http.StatusInternalServerError, "Failed to check in student to room"}
	}

	// Update last_activity on the active group to prevent session timeout
	// while staff are actively using the web check-in feature
	if activityErr := rs.ActiveService.UpdateSessionActivity(ctx, checkinCtx.request.ActiveGroupID); activityErr != nil {
		// Log but don't fail - the visit was created successfully
		rs.getLogger().WarnContext(ctx, "failed to update session activity after check-in",
			slog.Int64("active_group_id", checkinCtx.request.ActiveGroupID),
			slog.String("error", activityErr.Error()),
		)
	}

	return visit, nil
}

// respondCheckinSuccess sends the success response for check-in
func (rs *Resource) respondCheckinSuccess(w http.ResponseWriter, r *http.Request, ctx context.Context, visit *activeModels.Visit, checkinCtx *checkinContext) {
	responseData := map[string]interface{}{
		"student_id":      checkinCtx.studentID,
		"action":          "checked_in",
		"visit_id":        visit.ID,
		"active_group_id": checkinCtx.request.ActiveGroupID,
		"room_id":         checkinCtx.activeGroup.RoomID,
	}

	// Try to get updated attendance status for response
	updatedAttendance, statusErr := rs.ActiveService.GetStudentAttendanceStatus(ctx, checkinCtx.studentID)
	if statusErr != nil {
		rs.getLogger().WarnContext(ctx, "failed to get updated attendance status after checkin",
			slog.Int64("student_id", checkinCtx.studentID),
			slog.String("error", statusErr.Error()),
		)
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
