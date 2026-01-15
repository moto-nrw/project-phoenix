package active

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// ===== Active Group Handlers =====

// listActiveGroups handles listing all active groups
func (rs *Resource) listActiveGroups(w http.ResponseWriter, r *http.Request) {
	queryOptions := rs.parseActiveGroupQueryParams(r)

	groups, err := rs.ActiveService.ListActiveGroups(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	includeRelations := r.URL.Query().Get("active") == "true" || r.URL.Query().Get("is_active") == "true"
	if includeRelations && len(groups) > 0 {
		rs.loadActiveGroupRelations(r, groups)
	}

	responses := make([]ActiveGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newActiveGroupResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Active groups retrieved successfully")
}

// parseActiveGroupQueryParams parses query parameters for active groups
func (rs *Resource) parseActiveGroupQueryParams(r *http.Request) *base.QueryOptions {
	queryOptions := base.NewQueryOptions()

	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		if isActive {
			queryOptions.Filter.IsNull("end_time")
		} else {
			queryOptions.Filter.IsNotNull("end_time")
		}
	}

	return queryOptions
}

// loadActiveGroupRelations loads rooms and supervisors for active groups
func (rs *Resource) loadActiveGroupRelations(r *http.Request, groups []*active.Group) {
	roomMap := rs.loadRoomsMap(r, groups)
	supervisorMap := rs.loadActiveSupervisorsMap(r, groups)

	for _, group := range groups {
		if supervisors, ok := supervisorMap[group.ID]; ok {
			group.Supervisors = supervisors
		}
		if room, ok := roomMap[group.RoomID]; ok {
			group.Room = room
		}
	}
}

// loadRoomsMap loads rooms and returns a map of room ID to room
func (rs *Resource) loadRoomsMap(r *http.Request, groups []*active.Group) map[int64]*facilities.Room {
	roomIDs := rs.collectUniqueRoomIDs(groups)
	if len(roomIDs) == 0 {
		return make(map[int64]*facilities.Room)
	}

	roomMap, err := rs.FacilitiesService.GetRoomsByIDs(r.Context(), roomIDs)
	if err != nil {
		log.Printf("Error loading rooms: %v", err)
		return make(map[int64]*facilities.Room)
	}

	return roomMap
}

// collectUniqueRoomIDs collects unique room IDs from groups
func (rs *Resource) collectUniqueRoomIDs(groups []*active.Group) []int64 {
	roomIDs := make([]int64, 0, len(groups))
	roomIDMap := make(map[int64]bool)

	for _, group := range groups {
		if group.RoomID > 0 && !roomIDMap[group.RoomID] {
			roomIDs = append(roomIDs, group.RoomID)
			roomIDMap[group.RoomID] = true
		}
	}

	return roomIDs
}

// loadActiveSupervisorsMap loads supervisors and returns a map of group ID to active supervisors
func (rs *Resource) loadActiveSupervisorsMap(r *http.Request, groups []*active.Group) map[int64][]*active.GroupSupervisor {
	groupIDs := make([]int64, len(groups))
	for i, group := range groups {
		groupIDs[i] = group.ID
	}

	allSupervisors, err := rs.ActiveService.FindSupervisorsByActiveGroupIDs(r.Context(), groupIDs)
	if err != nil {
		log.Printf("Error loading supervisors: %v", err)
		return make(map[int64][]*active.GroupSupervisor)
	}

	activeSupervisors := make([]*active.GroupSupervisor, 0, len(allSupervisors))
	for _, supervisor := range allSupervisors {
		if supervisor.IsActive() {
			activeSupervisors = append(activeSupervisors, supervisor)
		}
	}

	supervisorMap := make(map[int64][]*active.GroupSupervisor)
	for _, supervisor := range activeSupervisors {
		supervisorMap[supervisor.GroupID] = append(supervisorMap[supervisor.GroupID], supervisor)
	}

	return supervisorMap
}

// getActiveGroup handles getting an active group by ID
func (rs *Resource) getActiveGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidActiveGroupID)))
		return
	}

	// Get active group
	group, err := rs.ActiveService.GetActiveGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Prepare response
	response := newActiveGroupResponse(group)

	common.Respond(w, r, http.StatusOK, response, "Active group retrieved successfully")
}

// getActiveGroupsByRoom handles getting active groups by room ID
func (rs *Resource) getActiveGroupsByRoom(w http.ResponseWriter, r *http.Request) {
	// Parse room ID from URL
	roomID, err := common.ParseIDParam(r, "roomId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid room ID")))
		return
	}

	// Get active groups for room
	groups, err := rs.ActiveService.FindActiveGroupsByRoomID(r.Context(), roomID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]ActiveGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newActiveGroupResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Room active groups retrieved successfully")
}

// getActiveGroupsByGroup handles getting active groups by group ID
func (rs *Resource) getActiveGroupsByGroup(w http.ResponseWriter, r *http.Request) {
	// Parse group ID from URL
	groupID, err := common.ParseIDParam(r, "groupId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidGroupID)))
		return
	}

	// Get active groups for group
	groups, err := rs.ActiveService.FindActiveGroupsByGroupID(r.Context(), groupID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]ActiveGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newActiveGroupResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Group active sessions retrieved successfully")
}

// getActiveGroupVisits handles getting visits for an active group
func (rs *Resource) getActiveGroupVisits(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidActiveGroupID)))
		return
	}

	// Get active group with visits
	group, err := rs.ActiveService.GetActiveGroupWithVisits(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]VisitResponse, 0, len(group.Visits))
	for _, visit := range group.Visits {
		responses = append(responses, newVisitResponse(visit))
	}

	common.Respond(w, r, http.StatusOK, responses, "Active group visits retrieved successfully")
}

// getActiveGroupVisitsWithDisplay handles getting visits with student display data in one query (optimized for SSE)
func (rs *Resource) getActiveGroupVisitsWithDisplay(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidActiveGroupID)))
		return
	}

	staff, err := rs.extractStaffFromRequest(w, r)
	if err != nil {
		return
	}

	if rs.verifyStaffSupervisionAccess(w, r, staff.ID, id) != nil {
		return
	}

	visits, err := rs.ActiveService.GetVisitsWithDisplayData(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	responses := rs.convertVisitsToDisplayResponses(visits)
	common.Respond(w, r, http.StatusOK, responses, "Active group visits with display data retrieved successfully")
}

// extractStaffFromRequest extracts staff information from JWT claims
func (rs *Resource) extractStaffFromRequest(w http.ResponseWriter, r *http.Request) (*users.Staff, error) {
	claims := jwt.ClaimsFromCtx(r.Context())

	person, err := rs.PersonService.FindByAccountID(r.Context(), int64(claims.ID))
	if err != nil || person == nil {
		common.RenderError(w, r, ErrorUnauthorized(errors.New("account not found")))
		return nil, errors.New("account not found")
	}

	staff, err := rs.PersonService.GetStaffByPersonID(r.Context(), person.ID)
	if err != nil || staff == nil {
		common.RenderError(w, r, ErrorForbidden(errors.New("user is not a staff member")))
		return nil, errors.New("user is not a staff member")
	}

	return staff, nil
}

// verifyStaffSupervisionAccess verifies staff has permission to view an active group
func (rs *Resource) verifyStaffSupervisionAccess(w http.ResponseWriter, r *http.Request, staffID int64, activeGroupID int64) error {
	supervisions, err := rs.ActiveService.GetStaffActiveSupervisions(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return err
	}

	hasPermission := false
	for _, supervision := range supervisions {
		if supervision.GroupID == activeGroupID {
			hasPermission = true
			break
		}
	}

	if !hasPermission {
		common.RenderError(w, r, ErrorForbidden(errors.New("not authorized to view this group")))
		return errors.New("not authorized")
	}

	_, err = rs.ActiveService.GetActiveGroup(r.Context(), activeGroupID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return err
	}

	return nil
}

// convertVisitsToDisplayResponses converts service visit data to API responses
func (rs *Resource) convertVisitsToDisplayResponses(visits []activeSvc.VisitWithDisplayData) []VisitWithDisplayDataResponse {
	responses := make([]VisitWithDisplayDataResponse, 0, len(visits))
	for _, v := range visits {
		studentName := v.FirstName + " " + v.LastName
		responses = append(responses, VisitWithDisplayDataResponse{
			ID:            v.VisitID,
			StudentID:     v.StudentID,
			ActiveGroupID: v.ActiveGroupID,
			CheckInTime:   v.EntryTime,
			CheckOutTime:  v.ExitTime,
			IsActive:      v.ExitTime == nil,
			StudentName:   studentName,
			SchoolClass:   v.SchoolClass,
			GroupName:     v.OGSGroupName,
			CreatedAt:     v.CreatedAt,
			UpdatedAt:     v.UpdatedAt,
		})
	}
	return responses
}

// getActiveGroupSupervisors handles getting supervisors for an active group
func (rs *Resource) getActiveGroupSupervisors(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidActiveGroupID)))
		return
	}

	// Get active group with supervisors
	group, err := rs.ActiveService.GetActiveGroupWithSupervisors(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]SupervisorResponse, 0, len(group.Supervisors))
	for _, supervisor := range group.Supervisors {
		responses = append(responses, newSupervisorResponse(supervisor))
	}

	common.Respond(w, r, http.StatusOK, responses, "Active group supervisors retrieved successfully")
}

// createActiveGroup handles creating a new active group
func (rs *Resource) createActiveGroup(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &ActiveGroupRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Create active group
	group := &active.Group{
		GroupID:   req.GroupID,
		RoomID:    req.RoomID,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	}

	// Create active group
	if err := rs.ActiveService.CreateActiveGroup(r.Context(), group); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the created active group
	createdGroup, err := rs.ActiveService.GetActiveGroup(r.Context(), group.ID)
	if err != nil {
		// Still return success but with the basic group info
		response := newActiveGroupResponse(group)
		common.Respond(w, r, http.StatusCreated, response, "Active group created successfully")
		return
	}

	// Return the active group with all details
	response := newActiveGroupResponse(createdGroup)
	common.Respond(w, r, http.StatusCreated, response, "Active group created successfully")
}

// updateActiveGroup handles updating an active group
func (rs *Resource) updateActiveGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidActiveGroupID)))
		return
	}

	// Parse request
	req := &ActiveGroupRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing active group
	existing, err := rs.ActiveService.GetActiveGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Update fields
	existing.GroupID = req.GroupID
	existing.RoomID = req.RoomID
	existing.StartTime = req.StartTime
	existing.EndTime = req.EndTime

	// Update active group
	if err := rs.ActiveService.UpdateActiveGroup(r.Context(), existing); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated active group
	updatedGroup, err := rs.ActiveService.GetActiveGroup(r.Context(), id)
	if err != nil {
		// Still return success but with the basic group info
		response := newActiveGroupResponse(existing)
		common.Respond(w, r, http.StatusOK, response, "Active group updated successfully")
		return
	}

	// Return the updated active group with all details
	response := newActiveGroupResponse(updatedGroup)
	common.Respond(w, r, http.StatusOK, response, "Active group updated successfully")
}

// deleteActiveGroup handles deleting an active group
func (rs *Resource) deleteActiveGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidActiveGroupID)))
		return
	}

	// Delete active group
	if err := rs.ActiveService.DeleteActiveGroup(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Active group deleted successfully")
}

// endActiveGroup handles ending an active group session
func (rs *Resource) endActiveGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidActiveGroupID)))
		return
	}

	// End active group session
	if err := rs.ActiveService.EndActiveGroupSession(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated active group
	updatedGroup, err := rs.ActiveService.GetActiveGroup(r.Context(), id)
	if err != nil {
		common.Respond(w, r, http.StatusOK, nil, "Active group session ended successfully")
		return
	}

	// Return the updated active group
	response := newActiveGroupResponse(updatedGroup)
	common.Respond(w, r, http.StatusOK, response, "Active group session ended successfully")
}

// ======== Unclaimed Groups Management (Deviceless Claiming) ========

// listUnclaimedGroups returns all active groups that have no supervisors
// This is used for deviceless rooms like Schulhof where teachers claim via frontend
func (rs *Resource) listUnclaimedGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := rs.ActiveService.GetUnclaimedActiveGroups(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, groups, "Unclaimed groups retrieved successfully")
}

// claimGroup allows authenticated staff to claim supervision of an active group
func (rs *Resource) claimGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get group ID from URL
	groupIDStr := chi.URLParam(r, "id")
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid group ID")
		return
	}

	// Get authenticated user from JWT token
	claims := jwt.ClaimsFromCtx(ctx)
	if claims.ID == 0 {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Get person from account ID
	person, err := rs.PersonService.FindByAccountID(ctx, int64(claims.ID))
	if err != nil || person == nil {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Account not found")
		return
	}

	// Get staff record from person
	staff, err := rs.PersonService.GetStaffByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Staff authentication required")
		return
	}

	// Claim the group (default role: "supervisor")
	supervisor, err := rs.ActiveService.ClaimActiveGroup(ctx, groupID, staff.ID, "supervisor")
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, supervisor, "Successfully claimed supervision")
}
