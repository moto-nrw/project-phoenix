package active

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
)

// createScheduledCheckout creates a new scheduled checkout for a student
func (rs *Resource) createScheduledCheckout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user from JWT context
	userClaims := jwt.ClaimsFromCtx(ctx)
	if userClaims.ID == 0 {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Parse request body
	var req struct {
		StudentID    int64  `json:"student_id"`
		ScheduledFor string `json:"scheduled_for"` // Accept as string first
		Reason       string `json:"reason,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Parse the time string
	scheduledTime, err := time.Parse(time.RFC3339, req.ScheduledFor)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid scheduled time format: "+err.Error())
		return
	}

	// Validate required fields
	if req.StudentID == 0 {
		common.RespondWithError(w, r, http.StatusBadRequest, "Student ID is required")
		return
	}

	// Get the person and staff info for the current user
	person, err := rs.PersonService.FindByAccountID(ctx, int64(userClaims.ID))
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get user information")
		return
	}

	if person == nil {
		common.RespondWithError(w, r, http.StatusForbidden, "User is not associated with a person record")
		return
	}

	staff, err := rs.PersonService.StaffRepository().FindByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		common.RespondWithError(w, r, http.StatusForbidden, "User is not a staff member")
		return
	}

	// Check if the user is authorized to schedule checkout for this student
	// Only education group teachers can schedule checkouts for their students
	hasAccess, err := rs.ActiveService.CheckTeacherStudentAccess(ctx, staff.ID, req.StudentID)
	if err != nil || !hasAccess {
		common.RespondWithError(w, r, http.StatusForbidden, "You are not authorized to schedule checkout for this student")
		return
	}

	// Create scheduled checkout
	checkout := &active.ScheduledCheckout{
		StudentID:    req.StudentID,
		ScheduledBy:  staff.ID,
		ScheduledFor: scheduledTime,
		Reason:       req.Reason,
	}

	if err := rs.ActiveService.CreateScheduledCheckout(ctx, checkout); err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to create scheduled checkout")
		return
	}

	common.RespondWithJSON(w, r, http.StatusCreated, map[string]interface{}{
		"status":  "success",
		"data":    checkout,
		"message": "Scheduled checkout created successfully",
	})
}

// getScheduledCheckout retrieves a scheduled checkout by ID
func (rs *Resource) getScheduledCheckout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid checkout ID")
		return
	}

	// Get scheduled checkout
	checkout, err := rs.ActiveService.GetScheduledCheckout(ctx, id)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get scheduled checkout")
		return
	}

	if checkout == nil {
		common.RespondWithError(w, r, http.StatusNotFound, "Scheduled checkout not found")
		return
	}

	common.RespondWithJSON(w, r, http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   checkout,
	})
}

// cancelScheduledCheckout cancels a scheduled checkout
func (rs *Resource) cancelScheduledCheckout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user from JWT context
	userClaims := jwt.ClaimsFromCtx(ctx)
	if userClaims.ID == 0 {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Resolve staff member performing the cancellation
	person, err := rs.PersonService.FindByAccountID(ctx, int64(userClaims.ID))
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get user information")
		return
	}

	if person == nil {
		common.RespondWithError(w, r, http.StatusForbidden, "User is not associated with a person record")
		return
	}

	staff, err := rs.PersonService.StaffRepository().FindByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		common.RespondWithError(w, r, http.StatusForbidden, "User is not a staff member")
		return
	}

	// Get ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid checkout ID")
		return
	}

	// Cancel scheduled checkout
	if err := rs.ActiveService.CancelScheduledCheckout(ctx, id, staff.ID); err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to cancel scheduled checkout")
		return
	}

	common.RespondWithJSON(w, r, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Scheduled checkout cancelled successfully",
	})
}

// getStudentScheduledCheckouts retrieves all scheduled checkouts for a student
func (rs *Resource) getStudentScheduledCheckouts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get student ID from URL
	studentIDStr := chi.URLParam(r, "studentId")
	studentID, err := strconv.ParseInt(studentIDStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid student ID")
		return
	}

	// Get scheduled checkouts
	checkouts, err := rs.ActiveService.GetStudentScheduledCheckouts(ctx, studentID)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get scheduled checkouts")
		return
	}

	common.RespondWithJSON(w, r, http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   checkouts,
	})
}

// getPendingScheduledCheckout retrieves the pending scheduled checkout for a student
func (rs *Resource) getPendingScheduledCheckout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get student ID from URL
	studentIDStr := chi.URLParam(r, "studentId")
	studentID, err := strconv.ParseInt(studentIDStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid student ID")
		return
	}

	// Get pending scheduled checkout
	checkout, err := rs.ActiveService.GetPendingScheduledCheckout(ctx, studentID)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to get pending scheduled checkout")
		return
	}

	if checkout == nil {
		common.RespondWithJSON(w, r, http.StatusOK, map[string]interface{}{
			"status":  "success",
			"data":    nil,
			"message": "No pending scheduled checkout found",
		})
		return
	}

	common.RespondWithJSON(w, r, http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   checkout,
	})
}

// processScheduledCheckouts processes all due scheduled checkouts
func (rs *Resource) processScheduledCheckouts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Process due checkouts
	result, err := rs.ActiveService.ProcessDueScheduledCheckouts(ctx)
	if err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to process scheduled checkouts")
		return
	}

	status := http.StatusOK
	if !result.Success {
		status = http.StatusPartialContent
	}

	common.RespondWithJSON(w, r, status, map[string]interface{}{
		"status":  "success",
		"data":    result,
		"message": fmt.Sprintf("Processed %d scheduled checkouts", result.CheckoutsExecuted),
	})
}
