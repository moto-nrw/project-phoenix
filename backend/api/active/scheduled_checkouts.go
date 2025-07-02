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
	
	// For now, use the account ID as the staff ID
	// TODO: Properly resolve staff ID from account
	staffID := int64(userClaims.ID)

	// Parse request body
	var req struct {
		StudentID    int64  `json:"student_id"`
		ScheduledFor string `json:"scheduled_for"` // Accept as string first
		Reason       string `json:"reason,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid request body: " + err.Error())
		return
	}
	
	// Parse the time string
	scheduledTime, err := time.Parse(time.RFC3339, req.ScheduledFor)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid scheduled time format: " + err.Error())
		return
	}

	// Validate required fields
	if req.StudentID == 0 {
		common.RespondWithError(w, r, http.StatusBadRequest, "Student ID is required")
		return
	}

	// Check if the user is authorized to schedule checkout for this student
	// We need to verify they're supervising the student's current group
	currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(ctx, req.StudentID)
	if err != nil {
		// Student might not be checked in, but we can still schedule a checkout
		// In this case, we'll skip the authorization check for now
		// TODO: Check if user supervises any of the student's groups
	} else {
		// Student is checked in, verify authorization
		if currentVisit.ActiveGroupID != 0 {
			activeGroup, err := rs.ActiveService.GetActiveGroupWithSupervisors(ctx, currentVisit.ActiveGroupID)
			if err != nil {
				common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to verify authorization")
				return
			}

			// Check if user is a supervisor or teacher
			isAuthorized := false
			for _, supervisor := range activeGroup.Supervisors {
				if supervisor.Staff != nil && supervisor.Staff.Person != nil && 
				   supervisor.Staff.Person.AccountID != nil && 
				   *supervisor.Staff.Person.AccountID == int64(userClaims.ID) {
					isAuthorized = true
					break
				}
			}

			// TODO: Also check if user is a teacher of the education group
			// This requires loading the education group information

			if !isAuthorized {
				common.RespondWithError(w, r, http.StatusForbidden, "You are not authorized to schedule checkout for this student")
				return
			}
		}
	}
	
	// Create scheduled checkout
	checkout := &active.ScheduledCheckout{
		StudentID:    req.StudentID,
		ScheduledBy:  staffID,
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
	// TODO: Extract current user from context
	// currentUser := r.Context().Value("user").(*users.TokenUser)

	// Get ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid checkout ID")
		return
	}

	// Cancel scheduled checkout
	if err := rs.ActiveService.CancelScheduledCheckout(ctx, id, 1); err != nil { // TODO: Get StaffID from current user
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
			"status": "success",
			"data":   nil,
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

