package active

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// ===== Combined Group Handlers =====

// listCombinedGroups handles listing all combined groups
func (rs *Resource) listCombinedGroups(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	queryOptions := studentpresence.CombinedGroupFilter{}

	// Get active status filter
	// Note: active.combined_groups doesn't have is_active column, use "active_only" filter
	// which the service/repository interprets as end_time IS NULL OR end_time > NOW()
	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		queryOptions.Active = &isActive
	}

	// Get combined groups
	groups, err := rs.listPresenceCombinations(r.Context(), queryOptions, "ListCombinedGroups")
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]CombinedGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newPresenceCombinationResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Combined groups retrieved successfully")
}

// getActiveCombinedGroups handles getting all active combined groups
func (rs *Resource) getActiveCombinedGroups(w http.ResponseWriter, r *http.Request) {
	// Get active combined groups
	groups, err := rs.listPresenceCombinations(r.Context(), studentpresence.CombinedGroupFilter{OpenOnly: true}, "FindActiveCombinedGroups")
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]CombinedGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newPresenceCombinationResponse(group))
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
	group, err := rs.presenceCombination(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Prepare response
	response := newPresenceCombinationResponse(group)

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
	_, groups, err := rs.presenceCombinationGroups(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]ActiveGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newPresenceLiveGroupResponse(group))
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

	// Create combined group atomically with all group mappings
	group := studentpresence.CombinedGroup{
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	}

	created, err := rs.Operations.CreateCombination(r.Context(), group, req.GroupIDs)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the created combined group with all groups
	combination, groups, err := rs.presenceCombinationGroups(r.Context(), created.ID)
	if err != nil {
		// Still return success but with the basic group info
		response := newPresenceCombinationResponse(created)
		common.Respond(w, r, http.StatusCreated, response, "Combined group created successfully")
		return
	}

	// Return the combined group with all details
	response := newCombinationWithGroupsResponse(combination, groups)
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
	existing, err := rs.presenceCombination(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Update fields
	existing.StartTime = req.StartTime
	existing.EndTime = req.EndTime

	// Update combined group
	revised, err := rs.Operations.AmendCombination(r.Context(), existing)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated combined group
	updatedGroup, err := rs.presenceCombination(r.Context(), id)
	if err != nil {
		// Still return success but with the basic group info
		response := newPresenceCombinationResponse(revised)
		common.Respond(w, r, http.StatusOK, response, "Combined group updated successfully")
		return
	}

	// Return the updated combined group with all details
	response := newPresenceCombinationResponse(updatedGroup)
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
	if err := rs.Operations.RemoveCombination(r.Context(), id); err != nil {
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
	if err := rs.Operations.CloseCombination(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated combined group
	updatedGroup, err := rs.presenceCombination(r.Context(), id)
	if err != nil {
		common.Respond(w, r, http.StatusOK, nil, "Combined group ended successfully")
		return
	}

	// Return the updated combined group
	response := newPresenceCombinationResponse(updatedGroup)
	common.Respond(w, r, http.StatusOK, response, "Combined group ended successfully")
}
