package active

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// ===== Supervisor Handlers =====

// listSupervisors handles listing all group supervisors
func (rs *Resource) listSupervisors(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	queryOptions := base.NewQueryOptions()

	// Get active status filter
	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		queryOptions.Filter.Equal("is_active", isActive)
	}

	// Get supervisors
	supervisors, err := rs.ActiveService.ListGroupSupervisors(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]SupervisorResponse, 0, len(supervisors))
	for _, supervisor := range supervisors {
		responses = append(responses, newSupervisorResponse(supervisor))
	}

	common.Respond(w, r, http.StatusOK, responses, "Supervisors retrieved successfully")
}

// getSupervisor handles getting a group supervisor by ID
func (rs *Resource) getSupervisor(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidSupervisorID)))
		return
	}

	// Get supervisor
	supervisor, err := rs.ActiveService.GetGroupSupervisor(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Prepare response
	response := newSupervisorResponse(supervisor)

	common.Respond(w, r, http.StatusOK, response, "Supervisor retrieved successfully")
}

// getStaffSupervisions handles getting supervisions for a staff member
func (rs *Resource) getStaffSupervisions(w http.ResponseWriter, r *http.Request) {
	// Parse staff ID from URL
	staffID, err := common.ParseIDParam(r, "staffId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid staff ID")))
		return
	}

	// Get supervisions for staff
	supervisors, err := rs.ActiveService.FindSupervisorsByStaffID(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]SupervisorResponse, 0, len(supervisors))
	for _, supervisor := range supervisors {
		responses = append(responses, newSupervisorResponse(supervisor))
	}

	common.Respond(w, r, http.StatusOK, responses, "Staff supervisions retrieved successfully")
}

// getStaffActiveSupervisions handles getting active supervisions for a staff member
func (rs *Resource) getStaffActiveSupervisions(w http.ResponseWriter, r *http.Request) {
	// Parse staff ID from URL
	staffID, err := common.ParseIDParam(r, "staffId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid staff ID")))
		return
	}

	// Get active supervisions for staff
	supervisors, err := rs.ActiveService.GetStaffActiveSupervisions(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]SupervisorResponse, 0, len(supervisors))
	for _, supervisor := range supervisors {
		responses = append(responses, newSupervisorResponse(supervisor))
	}

	common.Respond(w, r, http.StatusOK, responses, "Staff active supervisions retrieved successfully")
}

// getSupervisorsByGroup handles getting supervisors for an active group
func (rs *Resource) getSupervisorsByGroup(w http.ResponseWriter, r *http.Request) {
	// Parse group ID from URL
	groupID, err := common.ParseIDParam(r, "groupId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidGroupID)))
		return
	}

	// Get supervisors for active group
	supervisors, err := rs.ActiveService.FindSupervisorsByActiveGroupID(r.Context(), groupID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]SupervisorResponse, 0, len(supervisors))
	for _, supervisor := range supervisors {
		responses = append(responses, newSupervisorResponse(supervisor))
	}

	common.Respond(w, r, http.StatusOK, responses, "Group supervisors retrieved successfully")
}

// createSupervisor handles creating a new group supervisor
func (rs *Resource) createSupervisor(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &SupervisorRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Create supervisor
	supervisor := &active.GroupSupervisor{
		StaffID:   req.StaffID,
		GroupID:   req.ActiveGroupID,
		Role:      "Supervisor", // Default role
		StartDate: req.StartTime,
		EndDate:   req.EndTime,
	}

	// Create supervisor
	if err := rs.ActiveService.CreateGroupSupervisor(r.Context(), supervisor); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the created supervisor
	createdSupervisor, err := rs.ActiveService.GetGroupSupervisor(r.Context(), supervisor.ID)
	if err != nil {
		// Still return success but with the basic supervisor info
		response := newSupervisorResponse(supervisor)
		common.Respond(w, r, http.StatusCreated, response, "Supervisor created successfully")
		return
	}

	// Return the supervisor with all details
	response := newSupervisorResponse(createdSupervisor)
	common.Respond(w, r, http.StatusCreated, response, "Supervisor created successfully")
}

// updateSupervisor handles updating a group supervisor
func (rs *Resource) updateSupervisor(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidSupervisorID)))
		return
	}

	// Parse request
	req := &SupervisorRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing supervisor
	existing, err := rs.ActiveService.GetGroupSupervisor(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Update fields
	existing.StaffID = req.StaffID
	existing.GroupID = req.ActiveGroupID
	existing.StartDate = req.StartTime
	existing.EndDate = req.EndTime

	// Update supervisor
	if err := rs.ActiveService.UpdateGroupSupervisor(r.Context(), existing); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated supervisor
	updatedSupervisor, err := rs.ActiveService.GetGroupSupervisor(r.Context(), id)
	if err != nil {
		// Still return success but with the basic supervisor info
		response := newSupervisorResponse(existing)
		common.Respond(w, r, http.StatusOK, response, "Supervisor updated successfully")
		return
	}

	// Return the updated supervisor with all details
	response := newSupervisorResponse(updatedSupervisor)
	common.Respond(w, r, http.StatusOK, response, "Supervisor updated successfully")
}

// deleteSupervisor handles deleting a group supervisor
func (rs *Resource) deleteSupervisor(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidSupervisorID)))
		return
	}

	// Delete supervisor
	if err := rs.ActiveService.DeleteGroupSupervisor(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Supervisor deleted successfully")
}

// endSupervision handles ending a supervision
func (rs *Resource) endSupervision(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidSupervisorID)))
		return
	}

	// End supervision
	if err := rs.ActiveService.EndSupervision(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated supervisor
	updatedSupervisor, err := rs.ActiveService.GetGroupSupervisor(r.Context(), id)
	if err != nil {
		common.Respond(w, r, http.StatusOK, nil, "Supervision ended successfully")
		return
	}

	// Return the updated supervisor
	response := newSupervisorResponse(updatedSupervisor)
	common.Respond(w, r, http.StatusOK, response, "Supervision ended successfully")
}
