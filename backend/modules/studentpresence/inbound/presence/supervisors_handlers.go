package presence

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// ===== Supervisor Handlers =====

// listSupervisors handles listing all group supervisors
func (rs *Resource) listSupervisors(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	filter := studentpresence.GroupSupervisionFilter{}

	// Get active status filter
	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		day := timezone.TodayDate().String()
		if activeStr == "true" || activeStr == "1" {
			filter.ActiveOn = &day
		} else {
			filter.EndedBy = &day
		}
	}

	// Get supervisors
	responses, err := rs.presenceSupervisionResponses(r.Context(), filter, "ListGroupSupervisors")
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
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
	response, err := rs.presenceSupervisor(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

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
	day := timezone.TodayDate().String()
	responses, err := rs.presenceSupervisionResponses(r.Context(), studentpresence.GroupSupervisionFilter{StaffID: &staffID, ActiveOn: &day}, "FindSupervisorsByStaffID")
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
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
	day := timezone.TodayDate().String()
	responses, err := rs.presenceSupervisionResponses(r.Context(), studentpresence.GroupSupervisionFilter{StaffID: &staffID, ActiveOn: &day}, "GetStaffActiveSupervisions")
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	filtered := responses[:0]
	for _, response := range responses {
		if response.IsActive {
			filtered = append(filtered, response)
		}
	}
	responses = filtered

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
	day := timezone.TodayDate().String()
	responses, err := rs.presenceSupervisionResponses(r.Context(), studentpresence.GroupSupervisionFilter{GroupIDs: []int64{groupID}, ActiveOn: &day}, "FindSupervisorsByActiveGroupID")
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
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
	supervisor := studentpresence.GroupSupervision{
		StaffID:   req.StaffID,
		GroupID:   req.ActiveGroupID,
		Role:      "Supervisor", // Default role
		StartDate: timezone.DateFromTime(req.StartTime).String(),
		EndDate:   supervisorEndDate(req.EndTime),
	}

	// Create supervisor
	created, err := rs.Operations.AssignSupervision(r.Context(), supervisor)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the created supervisor
	response, err := rs.presenceSupervisor(r.Context(), created.ID)
	if err != nil {
		// Still return success but with the basic supervisor info
		common.Respond(w, r, http.StatusCreated, supervisionRowResponse(created), "Supervisor created successfully")
		return
	}

	// Return the supervisor with all details
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
	existing, err := rs.presenceSupervisionRow(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Update fields
	existing.StaffID = req.StaffID
	existing.GroupID = req.ActiveGroupID
	existing.StartDate = timezone.DateFromTime(req.StartTime).String()
	existing.EndDate = supervisorEndDate(req.EndTime)

	// Update supervisor
	if err := rs.Operations.AmendSupervision(r.Context(), existing); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated supervisor
	response, err := rs.presenceSupervisor(r.Context(), id)
	if err != nil {
		// Still return success but with the basic supervisor info
		common.Respond(w, r, http.StatusOK, supervisionRowResponse(existing), "Supervisor updated successfully")
		return
	}

	// Return the updated supervisor with all details
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
	if err := rs.Operations.RemoveSupervisionRecord(r.Context(), id); err != nil {
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
	if err := rs.Operations.EndSupervision(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated supervisor
	response, err := rs.presenceSupervisor(r.Context(), id)
	if err != nil {
		common.Respond(w, r, http.StatusOK, nil, "Supervision ended successfully")
		return
	}

	// Return the updated supervisor
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
	groups, err := rs.listPresenceLiveGroups(ctx, studentpresence.LiveGroupFilter{})
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response using the same ActiveGroupResponse format
	responses := make([]ActiveGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newPresenceLiveGroupResponse(group))
	}
	// Preserve enrichment of the complete query result before filtering open sessions.
	if len(groups) > 0 {
		rs.loadActiveGroupRelations(ctx, groups, responses)
	}
	openResponses := responses[:0]
	for _, response := range responses {
		if response.IsActive {
			openResponses = append(openResponses, response)
		}
	}

	common.Respond(w, r, http.StatusOK, openResponses, "All active groups retrieved successfully")
}

// supervisorEndDate converts an optional wire instant into the optional
// end-date calendar day. nil stays nil — an open-ended supervision must
// never gain a zero-Date sentinel.
func supervisorEndDate(endTime *time.Time) *string {
	if endTime == nil {
		return nil
	}
	d := timezone.DateFromTime(*endTime).String()
	return &d
}
