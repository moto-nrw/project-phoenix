package active

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// ===== Combined Group Handlers =====

// listCombinedGroups handles listing all combined groups
func (rs *Resource) listCombinedGroups(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	queryOptions := base.NewQueryOptions()

	// Get active status filter
	// Note: active.combined_groups doesn't have is_active column, use "active_only" filter
	// which the service/repository interprets as end_time IS NULL OR end_time > NOW()
	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		queryOptions.Filter.Equal("active_only", isActive)
	}

	// Get combined groups
	groups, err := rs.ActiveService.ListCombinedGroups(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]CombinedGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newCombinedGroupResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Combined groups retrieved successfully")
}

// getActiveCombinedGroups handles getting all active combined groups
func (rs *Resource) getActiveCombinedGroups(w http.ResponseWriter, r *http.Request) {
	// Get active combined groups
	groups, err := rs.ActiveService.FindActiveCombinedGroups(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]CombinedGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newCombinedGroupResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Active combined groups retrieved successfully")
}

// getCombinedGroup handles getting a combined group by ID
func (rs *Resource) getCombinedGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidCombinedGroupID)))
		return
	}

	// Get combined group
	group, err := rs.ActiveService.GetCombinedGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Prepare response
	response := newCombinedGroupResponse(group)

	common.Respond(w, r, http.StatusOK, response, "Combined group retrieved successfully")
}

// getCombinedGroupGroups handles getting active groups in a combined group
func (rs *Resource) getCombinedGroupGroups(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidCombinedGroupID)))
		return
	}

	// Get combined group with groups
	combinedGroup, err := rs.ActiveService.GetCombinedGroupWithGroups(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]ActiveGroupResponse, 0, len(combinedGroup.ActiveGroups))
	for _, group := range combinedGroup.ActiveGroups {
		responses = append(responses, newActiveGroupResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Combined group's active groups retrieved successfully")
}

// createCombinedGroup handles creating a new combined group
func (rs *Resource) createCombinedGroup(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &CombinedGroupRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Create combined group
	group := &active.CombinedGroup{
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	}

	// Create combined group
	if err := rs.ActiveService.CreateCombinedGroup(r.Context(), group); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Add groups to the combined group if provided
	if len(req.GroupIDs) > 0 {
		for _, groupID := range req.GroupIDs {
			if rs.ActiveService.AddGroupToCombination(r.Context(), group.ID, groupID) != nil {
				// Log error but continue (see #554 for partial failure handling)
				continue
			}
		}
	}

	// Get the created combined group with all groups
	createdGroup, err := rs.ActiveService.GetCombinedGroupWithGroups(r.Context(), group.ID)
	if err != nil {
		// Still return success but with the basic group info
		response := newCombinedGroupResponse(group)
		common.Respond(w, r, http.StatusCreated, response, "Combined group created successfully")
		return
	}

	// Return the combined group with all details
	response := newCombinedGroupResponse(createdGroup)
	common.Respond(w, r, http.StatusCreated, response, "Combined group created successfully")
}

// updateCombinedGroup handles updating a combined group
func (rs *Resource) updateCombinedGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidCombinedGroupID)))
		return
	}

	// Parse request
	req := &CombinedGroupRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing combined group
	existing, err := rs.ActiveService.GetCombinedGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Update fields
	existing.StartTime = req.StartTime
	existing.EndTime = req.EndTime

	// Update combined group
	if err := rs.ActiveService.UpdateCombinedGroup(r.Context(), existing); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated combined group
	updatedGroup, err := rs.ActiveService.GetCombinedGroup(r.Context(), id)
	if err != nil {
		// Still return success but with the basic group info
		response := newCombinedGroupResponse(existing)
		common.Respond(w, r, http.StatusOK, response, "Combined group updated successfully")
		return
	}

	// Return the updated combined group with all details
	response := newCombinedGroupResponse(updatedGroup)
	common.Respond(w, r, http.StatusOK, response, "Combined group updated successfully")
}

// deleteCombinedGroup handles deleting a combined group
func (rs *Resource) deleteCombinedGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidCombinedGroupID)))
		return
	}

	// Delete combined group
	if err := rs.ActiveService.DeleteCombinedGroup(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Combined group deleted successfully")
}

// endCombinedGroup handles ending a combined group
func (rs *Resource) endCombinedGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidCombinedGroupID)))
		return
	}

	// End combined group
	if err := rs.ActiveService.EndCombinedGroup(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated combined group
	updatedGroup, err := rs.ActiveService.GetCombinedGroup(r.Context(), id)
	if err != nil {
		common.Respond(w, r, http.StatusOK, nil, "Combined group ended successfully")
		return
	}

	// Return the updated combined group
	response := newCombinedGroupResponse(updatedGroup)
	common.Respond(w, r, http.StatusOK, response, "Combined group ended successfully")
}
