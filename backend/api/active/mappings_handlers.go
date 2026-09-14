package active

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// ===== Group Mapping Handlers =====

// getGroupMappings handles getting mappings for an active group
func (rs *Resource) getGroupMappings(w http.ResponseWriter, r *http.Request) {
	// Parse group ID from URL
	groupID, err := common.ParseIDParam(r, "groupId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidGroupID)))
		return
	}

	// Get mappings for active group
	mappings, err := rs.presenceGroupMappings(r.Context(), studentpresence.GroupMappingFilter{ActiveGroupID: &groupID}, "GetGroupMappingsByActiveGroupID")
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]GroupMappingResponse, 0, len(mappings))
	for _, mapping := range mappings {
		responses = append(responses, newPresenceGroupMappingResponse(mapping))
	}

	common.Respond(w, r, http.StatusOK, responses, "Group mappings retrieved successfully")
}

// getCombinedGroupMappings handles getting mappings for a combined group
func (rs *Resource) getCombinedGroupMappings(w http.ResponseWriter, r *http.Request) {
	// Parse combined group ID from URL
	combinedID, err := common.ParseIDParam(r, "combinedId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidCombinedGroupID)))
		return
	}

	// Get mappings for combined group
	mappings, err := rs.presenceGroupMappings(r.Context(), studentpresence.GroupMappingFilter{CombinedGroupID: &combinedID}, "GetGroupMappingsByCombinedGroupID")
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]GroupMappingResponse, 0, len(mappings))
	for _, mapping := range mappings {
		responses = append(responses, newPresenceGroupMappingResponse(mapping))
	}

	common.Respond(w, r, http.StatusOK, responses, "Combined group mappings retrieved successfully")
}

// addGroupToCombination handles adding an active group to a combined group
func (rs *Resource) addGroupToCombination(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &GroupMappingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Add group to combination
	if err := rs.addPresenceGroupToCombination(r.Context(), req.CombinedGroupID, req.ActiveGroupID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the mappings for verification
	mappings, err := rs.presenceGroupMappings(r.Context(), studentpresence.GroupMappingFilter{CombinedGroupID: &req.CombinedGroupID}, "GetGroupMappingsByCombinedGroupID")
	if err != nil {
		common.Respond(w, r, http.StatusOK, nil, msgGroupAddedToCombination)
		return
	}

	// Find the newly created mapping
	var newMapping *studentpresence.GroupMapping
	for _, mapping := range mappings {
		if mapping.ActiveGroupID == req.ActiveGroupID {
			newMapping = &mapping
			break
		}
	}

	if newMapping == nil {
		common.Respond(w, r, http.StatusOK, nil, msgGroupAddedToCombination)
		return
	}

	// Return the mapping
	response := newPresenceGroupMappingResponse(*newMapping)
	common.Respond(w, r, http.StatusOK, response, msgGroupAddedToCombination)
}

// removeGroupFromCombination handles removing an active group from a combined group
func (rs *Resource) removeGroupFromCombination(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &GroupMappingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Remove group from combination
	if err := rs.removePresenceGroupFromCombination(r.Context(), req.CombinedGroupID, req.ActiveGroupID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Group removed from combination successfully")
}
