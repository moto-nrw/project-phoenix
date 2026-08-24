package active

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// ===== Supervisor Handlers =====

// listSupervisors handles listing all group supervisors
func (rs *Resource) listSupervisors(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	queryOptions := base.NewQueryOptions()

	// Get active status filter
	// Note: active.group_supervisors doesn't have is_active column, use "active_only" filter
	// which the service/repository interprets as end_date IS NULL OR end_date > NOW()
	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		queryOptions.Filter.Equal("active_only", isActive)
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

	// Create supervisor. The wire carries RFC3339 instants; only their
	// Berlin calendar days matter for the DATE columns.
	supervisor := &active.GroupSupervisor{
		StaffID:   req.StaffID,
		GroupID:   req.ActiveGroupID,
		Role:      "Supervisor", // Default role
		StartDate: timezone.DateFromTime(req.StartTime),
		EndDate:   supervisorEndDate(req.EndTime),
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
	existing.StartDate = timezone.DateFromTime(req.StartTime)
	existing.EndDate = supervisorEndDate(req.EndTime)

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

// getAllActiveSupervisions returns all active groups with room info for every
// caller the school-wide overview scope covers (#2380).
// Returns the same response format as /api/me/groups/supervised so the frontend
// can consume both endpoints identically — a 403 here is the client's signal
// to fall back to its own supervisions.
// GET /api/active/supervisors/all
func (rs *Resource) getAllActiveSupervisions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// The route already requires groups:read. The overview scope may broaden
	// WHICH groups the caller sees, but never replaces the permission check.
	if rs.SettingsService == nil {
		common.RenderError(w, r, ErrorForbidden(errors.New("operational group overview is not available")))
		return
	}
	if !rs.operationalOverview(ctx) {
		common.RenderError(w, r, ErrorForbidden(errors.New("all-group operational access is not enabled for this school")))
		return
	}

	// Get all active groups with room info (same format as /api/me/groups/supervised)
	groups, err := rs.ActiveService.ListActiveGroups(ctx, base.NewQueryOptions())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Load room relations for display
	if len(groups) > 0 {
		rs.loadActiveGroupRelations(r, groups)
	}

	// Build response using the same ActiveGroupResponse format
	responses := make([]ActiveGroupResponse, 0, len(groups))
	for _, group := range groups {
		if group.IsActive() {
			responses = append(responses, newActiveGroupResponse(group))
		}
	}

	common.Respond(w, r, http.StatusOK, responses, "All active groups retrieved successfully")
}

// supervisorEndDate converts an optional wire instant into the optional
// end-date calendar day. nil stays nil — an open-ended supervision must
// never gain a zero-Date sentinel.
func supervisorEndDate(endTime *time.Time) *timezone.Date {
	if endTime == nil {
		return nil
	}
	d := timezone.DateFromTime(*endTime)
	return &d
}
