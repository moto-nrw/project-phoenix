package activities

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// =============================================================================
// SUPERVISOR ASSIGNMENT HANDLERS
// =============================================================================

// getActivitySupervisors retrieves all supervisors for a specific activity
func (rs *Resource) getActivitySupervisors(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	// Get supervisors for the activity
	supervisors, err := rs.ActivityService.GetGroupSupervisors(r.Context(), activity.ID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Convert to response objects
	responses := make([]SupervisorResponse, 0, len(supervisors))
	for _, supervisor := range supervisors {
		if supervisor == nil {
			continue // Skip nil supervisors to prevent panic
		}
		responses = append(responses, newSupervisorResponse(supervisor))
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Supervisors for activity '%s' retrieved successfully", activity.Name))
}

// getAvailableSupervisors retrieves available supervisors for assignment
func (rs *Resource) getAvailableSupervisors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	specialization := r.URL.Query().Get("specialization")

	var supervisors []SupervisorResponse
	var err error

	if specialization != "" {
		supervisors, err = rs.fetchSupervisorsBySpecialization(ctx, specialization)
		if err != nil {
			common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to retrieve teachers")
			return
		}
	} else {
		supervisors, err = rs.fetchAllSupervisors(ctx)
		if err != nil {
			common.RespondWithError(w, r, http.StatusInternalServerError, "Failed to retrieve staff")
			return
		}
	}

	common.Respond(w, r, http.StatusOK, supervisors, "Available supervisors retrieved successfully")
}

// SupervisorRequest represents a supervisor assignment request
type SupervisorRequest struct {
	StaffID   int64 `json:"staff_id"`
	IsPrimary bool  `json:"is_primary"`
}

// Bind validates the supervisor request
func (req *SupervisorRequest) Bind(_ *http.Request) error {
	if req.StaffID <= 0 {
		return errors.New("staff ID is required and must be greater than 0")
	}
	return nil
}

// assignSupervisor assigns a supervisor to an activity
func (rs *Resource) assignSupervisor(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	// Parse request
	var req SupervisorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Validate request
	if err := req.Bind(r); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Assign supervisor
	supervisor, err := rs.ActivityService.AddSupervisor(r.Context(), activity.ID, req.StaffID, req.IsPrimary)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newSupervisorResponse(supervisor), "Supervisor assigned successfully")
}

// updateSupervisorRole updates a supervisor's role (primary/non-primary)
func (rs *Resource) updateSupervisorRole(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	supervisorID, ok := rs.parseSupervisorID(w, r)
	if !ok {
		return
	}

	// Parse request
	var req SupervisorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing supervisor
	supervisor, err := rs.ActivityService.GetSupervisor(r.Context(), supervisorID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Check if supervisor belongs to the specified activity
	if !rs.checkSupervisorOwnership(w, r, supervisor, activity.ID) {
		return
	}

	// If making this supervisor primary, use the service method to handle it properly
	if req.IsPrimary && !supervisor.IsPrimary {
		if err := rs.ActivityService.SetPrimarySupervisor(r.Context(), supervisorID); err != nil {
			common.RenderError(w, r, ErrorRenderer(err))
			return
		}
	} else if supervisor.IsPrimary != req.IsPrimary {
		// Only update if the primary status is changing
		supervisor.IsPrimary = req.IsPrimary
		if _, err := rs.ActivityService.UpdateSupervisor(r.Context(), supervisor); err != nil {
			common.RenderError(w, r, ErrorRenderer(err))
			return
		}
	}

	// Get the updated supervisor
	updatedSupervisor, err := rs.ActivityService.GetSupervisor(r.Context(), supervisorID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newSupervisorResponse(updatedSupervisor), "Supervisor role updated successfully")
}

// removeSupervisor removes a supervisor from an activity
func (rs *Resource) removeSupervisor(w http.ResponseWriter, r *http.Request) {
	activity, ok := rs.parseAndGetActivity(w, r)
	if !ok {
		return
	}

	supervisorID, ok := rs.parseSupervisorID(w, r)
	if !ok {
		return
	}

	// Get supervisor to verify ownership
	supervisor, err := rs.ActivityService.GetSupervisor(r.Context(), supervisorID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Check if supervisor belongs to the specified activity
	if !rs.checkSupervisorOwnership(w, r, supervisor, activity.ID) {
		return
	}

	// Delete supervisor
	if err := rs.ActivityService.DeleteSupervisor(r.Context(), supervisorID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Supervisor removed successfully")
}
