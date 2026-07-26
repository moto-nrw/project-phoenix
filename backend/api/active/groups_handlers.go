package active

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	configService "github.com/moto-nrw/project-phoenix/services/config"
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
	roomMap := make(map[int64]*facilities.Room)

	if len(roomIDs) > 0 {
		rooms, err := rs.ActiveService.GetRoomsByIDs(r.Context(), roomIDs)
		if err == nil {
			for _, room := range rooms {
				roomMap[room.ID] = room
			}
		}
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
		slog.Default().Error("failed to load supervisors", slog.String("error", err.Error()))
		return make(map[int64][]*active.GroupSupervisor)
	}

	now := time.Now()
	activeSupervisors := make([]*active.GroupSupervisor, 0, len(allSupervisors))
	for _, supervisor := range allSupervisors {
		if activeService.IsSupervisorActive(supervisor, now) {
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

	// Admin overview and open-care both broaden the operational room scope.
	// Per-student GDPR fields below still use DetermineStudentAccess and are
	// never broadened by group mode.
	if !rs.isAdminWithSupervisionOverview(r) && !rs.openCareMode(r.Context()) {
		staff, err := rs.extractStaffFromRequest(w, r)
		if err != nil {
			return
		}

		if rs.verifyStaffSupervisionAccess(w, r, staff.ID, id) != nil {
			return
		}
	}

	results, err := rs.fetchVisitsWithDisplayData(r, id)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Resolve the caller's per-student access context once. Actual
	// arrival/pickup times are gated identically to planned times — a
	// caller who can't see a student's planned schedule (because they
	// don't supervise the student's education group) must not see the
	// real check-in/out clock either.
	access := common.DetermineStudentAccess(r, rs.UserContextService, rs.SettingsService, rs.getLogger())

	attendanceStatuses, err := rs.fetchAttendanceStatusesForVisits(r, results, access)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	photosEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyStudentPhotosEnabled, false, rs.getLogger())
	responses := rs.buildVisitDisplayResponses(results, attendanceStatuses, access, photosEnabled)
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

// isAdminWithSupervisionOverview checks if the current user is an admin with the
// admin_supervision_overview setting enabled for this tenant.
func (rs *Resource) isAdminWithSupervisionOverview(r *http.Request) bool {
	claims := jwt.ClaimsFromCtx(r.Context())
	if !claims.IsAdmin {
		return false
	}
	if rs.SettingsService == nil {
		return false
	}
	enabled, err := rs.SettingsService.ResolveBool(r.Context(), configModel.KeyAdminSupervisionOverview)
	if err != nil {
		return false
	}
	return enabled
}

// visitWithStudent aliases the repository read model (issue #584: the JOIN
// query moved into VisitRepository.FindActiveWithStudentDisplayByGroup).
type visitWithStudent = active.VisitWithStudentDisplay

// fetchVisitsWithDisplayData fetches visits with student display data
func (rs *Resource) fetchVisitsWithDisplayData(r *http.Request, activeGroupID int64) ([]visitWithStudent, error) {
	rows, err := rs.ActiveService.GetActiveGroupVisitsWithDisplay(r.Context(), activeGroupID)
	if err != nil {
		return nil, err
	}
	results := make([]visitWithStudent, 0, len(rows))
	for _, row := range rows {
		results = append(results, *row)
	}
	return results, nil
}

// fetchAttendanceStatusesForVisits fetches today's attendance status for the
// students in results, but only for the subset the caller has full access to.
// Students outside the caller's access scope are skipped entirely so the DB
// never returns their actual check-in/out times to this request.
func (rs *Resource) fetchAttendanceStatusesForVisits(r *http.Request, results []visitWithStudent, access *common.StudentAccessContext) (map[int64]*activeService.AttendanceStatus, error) {
	studentIDs := collectAuthorizedVisitStudentIDs(results, access)
	if len(studentIDs) == 0 {
		return map[int64]*activeService.AttendanceStatus{}, nil
	}

	return rs.ActiveService.GetStudentsAttendanceStatuses(r.Context(), studentIDs)
}

// collectAuthorizedVisitStudentIDs returns the unique student IDs from results
// for which the caller has full data access (admin, all_staff scope, or group
// supervisor). Used to scope the bulk attendance lookup so unauthorized rows
// never leave the DB.
func collectAuthorizedVisitStudentIDs(results []visitWithStudent, access *common.StudentAccessContext) []int64 {
	studentIDs := make([]int64, 0, len(results))
	seen := make(map[int64]struct{}, len(results))

	for _, result := range results {
		if _, ok := seen[result.StudentID]; ok {
			continue
		}
		if !access.HasFullAccessByGroupID(result.GroupID) {
			continue
		}
		seen[result.StudentID] = struct{}{}
		studentIDs = append(studentIDs, result.StudentID)
	}

	return studentIDs
}

// buildVisitDisplayResponses builds visit responses with display data.
// Actual arrival/pickup times are emitted only for students the caller has
// full data access to — the same gate planned times use on the bulk pickup
// and arrival endpoints. Other fields (name, school class, sick/excused) keep
// their existing group-level visibility.
//
// photosEnabled mirrors operations.student_photos_enabled. When false we
// skip photo_url for every row so an admin who turns the feature off
// after photos were uploaded actually suppresses them — matches the
// gate in api/students/response_helpers.go.
func (rs *Resource) buildVisitDisplayResponses(results []visitWithStudent, attendanceStatuses map[int64]*activeService.AttendanceStatus, access *common.StudentAccessContext, photosEnabled bool) []VisitWithDisplayDataResponse {
	responses := make([]VisitWithDisplayDataResponse, 0, len(results))
	for _, result := range results {
		studentName := result.FirstName + " " + result.LastName
		sick := false
		if result.Sick != nil {
			sick = *result.Sick
		}
		excused := false
		if result.Excused != nil {
			excused = *result.Excused
		}

		var actualArrival *string
		var actualPickup *string
		if access.HasFullAccessByGroupID(result.GroupID) {
			if attendanceStatus, ok := attendanceStatuses[result.StudentID]; ok && attendanceStatus != nil {
				actualArrival = timezone.FormatBerlinClock(attendanceStatus.CheckInTime)
				actualPickup = timezone.FormatBerlinClock(attendanceStatus.CheckOutTime)
			}
		}

		// Rewrite the raw /uploads/student-photos/{filename} path stored on
		// the row to the authenticated /api/students/{id}/photo/{filename}
		// proxy URL the browser uses (same logic as populatePhotoFields in
		// api/students/response_helpers.go). Storing the proxy URL inside
		// active.visits would couple two domains; rewriting here keeps the
		// JSON contract consistent across endpoints — same helper the
		// student response shaper uses, so a path-format change in one
		// place can't desync the two response payloads.
		//
		// Gate on access.HasFullAccessByGroupID(result.GroupID) — the same
		// predicate the actualArrival/actualPickup branch above uses, and
		// the same predicate serveStudentPhoto checks via
		// authorize.CanReadStudent. A caregiver may supervise this active
		// room without supervising the student's home education group;
		// without the gate, list responses would emit photo_url for those
		// rows and every avatar request would 403 in the byte-serve path,
		// leaving the UI with broken-image placeholders.
		photoURL := ""
		if photosEnabled && result.PhotoPath != nil &&
			access.HasFullAccessByGroupID(result.GroupID) {
			photoURL = common.BuildStudentPhotoServeURL(result.StudentID, *result.PhotoPath)
		}

		responses = append(responses, VisitWithDisplayDataResponse{
			ID:            result.VisitID,
			StudentID:     result.StudentID,
			ActiveGroupID: result.ActiveGroupID,
			CheckInTime:   result.EntryTime,
			CheckOutTime:  result.ExitTime,
			ActualArrival: actualArrival,
			ActualPickup:  actualPickup,
			IsActive:      result.ExitTime == nil,
			StudentName:   studentName,
			SchoolClass:   result.SchoolClass,
			GroupName:     result.OGSGroupName,
			Sick:          sick,
			SickSince:     result.SickSince,
			Excused:       excused,
			ExcusedSince:  result.ExcusedSince,
			PhotoURL:      photoURL,
			CreatedAt:     result.CreatedAt,
			UpdatedAt:     result.UpdatedAt,
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

	// Create active group. The CRUD endpoint always carries a template id
	// (validated in req.Bind as > 0), so GroupID is always set here —
	// spontaneous instance creation runs through a separate handler (WP-B9+).
	groupID := req.GroupID
	group := &active.Group{
		GroupID:   &groupID,
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

	// Update fields. See create handler comment on GroupID — the CRUD surface
	// always supplies a template id, so we wrap the value in a pointer.
	groupID := req.GroupID
	existing.GroupID = &groupID
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
