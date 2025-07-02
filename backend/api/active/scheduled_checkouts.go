package active

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/active"
)

// createScheduledCheckout creates a new scheduled checkout for a student
func (rs *Resource) createScheduledCheckout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// TODO: Extract staff ID from JWT token context
	// For now, using a placeholder staff ID
	staffID := int64(1) // This should be extracted from the authenticated user context

	// Parse request body
	var req struct {
		StudentID    int64     `json:"student_id"`
		ScheduledFor time.Time `json:"scheduled_for"`
		Reason       string    `json:"reason,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.StudentID == 0 {
		common.RespondWithError(w, r, http.StatusBadRequest, "Student ID is required")
		return
	}

	if req.ScheduledFor.IsZero() {
		common.RespondWithError(w, r, http.StatusBadRequest, "Scheduled time is required")
		return
	}

	// Create scheduled checkout
	checkout := &active.ScheduledCheckout{
		StudentID:    req.StudentID,
		ScheduledBy:  staffID,
		ScheduledFor: req.ScheduledFor,
		Reason:       req.Reason,
	}

	if err := rs.ActiveService.CreateScheduledCheckout(ctx, checkout); err != nil {
		common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to create scheduled checkout")
		return
	}

	common.RespondWithJSON(w, http.StatusCreated, map[string]interface{}{
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

	common.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
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

	common.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
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

	common.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
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
		common.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
			"status": "success",
			"data":   nil,
			"message": "No pending scheduled checkout found",
		})
		return
	}

	common.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
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

	common.RespondWithJSON(w, status, map[string]interface{}{
		"status":  "success",
		"data":    result,
		"message": fmt.Sprintf("Processed %d scheduled checkouts", result.CheckoutsExecuted),
	})
}

