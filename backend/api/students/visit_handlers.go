package students

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configService "github.com/moto-nrw/project-phoenix/services/config"
)

// getStudentCurrentLocation handles getting a student's current location with scheduled checkout info
func (rs *Resource) getStudentCurrentLocation(w http.ResponseWriter, r *http.Request) {
	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Get person details
	person, ok := rs.getPersonForStudent(w, r, student)
	if !ok {
		return
	}

	// Get group details if student has a group
	group := rs.getStudentGroup(r.Context(), student)

	// Determine if user has full access to student location details
	hasFullAccess := rs.checkStudentReadAccess(r, student)
	photosEnabled := configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyStudentPhotosEnabled, false, rs.Logger)

	// Build student response
	response := newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
		Student:       student,
		Person:        person,
		Group:         group,
		HasFullAccess: hasFullAccess,
		PhotosEnabled: photosEnabled,
	}, StudentResponseServices{
		ActiveService: rs.ActiveService,
		PersonService: rs.PersonService,
	})

	// Create location response structure
	locationResponse := struct {
		Location    string `json:"current_location"`
		CurrentRoom string `json:"current_room,omitempty"`
	}{
		Location: response.Location,
	}

	// If student is present and user has full access, try to get current room
	if hasFullAccess && response.Location == "Anwesend" {
		if currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(r.Context(), student.ID); err == nil && currentVisit != nil {
			if activeGroup, err := rs.ActiveService.GetActiveGroup(r.Context(), currentVisit.ActiveGroupID); err == nil && activeGroup != nil {
				// The room should be loaded as part of the active group
				if activeGroup.Room != nil {
					locationResponse.CurrentRoom = activeGroup.Room.Name
				}
			}
		}
	}

	common.Respond(w, r, http.StatusOK, locationResponse, "Student location retrieved successfully")
}

// checkGroupRoomAccessAuthorization verifies if the user can view student room status.
// Returns an error if unauthorized, nil if authorized.
//
// Grants access to: admins, any staff when gdpr.student_data_scope = all_staff,
// and the student's group supervisors. This endpoint is read-only.
func (rs *Resource) checkGroupRoomAccessAuthorization(r *http.Request, studentGroupID int64) error {
	if authorize.HasAdminWildcard(getPermissionsFromRequest(r)) {
		return nil
	}

	// Tenant-configurable read scope: when set to all_staff, any authenticated
	// staff member can view this information.
	scope := configService.ResolveStringOrDefault(
		r.Context(),
		rs.SettingsService,
		configModel.KeyStudentDataScope,
		configModel.StudentDataScopeGroupSupervisorsOnly,
		rs.Logger,
	)
	if scope == configModel.StudentDataScopeAllStaff {
		if staff, err := rs.UserContextService.GetCurrentStaff(r.Context()); err == nil && staff != nil {
			return nil
		}
		return errors.New("unauthorized to view student room status")
	}

	staff, err := rs.UserContextService.GetCurrentStaff(r.Context())
	if err != nil || staff == nil {
		return errors.New("unauthorized to view student room status")
	}

	educationGroups, err := rs.UserContextService.GetMyGroups(r.Context())
	if err != nil {
		return errors.New("you do not supervise this student's group")
	}

	for _, supervGroup := range educationGroups {
		if supervGroup.ID == studentGroupID {
			return nil
		}
	}

	return errors.New("you do not supervise this student's group")
}

// buildGroupRoomResponse constructs the response for in-group-room check
func buildGroupRoomResponse(activeGroup *active.Group, groupRoomID int64, groupRoomName string) map[string]interface{} {
	inGroupRoom := activeGroup.RoomID == groupRoomID
	response := map[string]interface{}{
		"in_group_room":   inGroupRoom,
		"group_room_id":   groupRoomID,
		"current_room_id": activeGroup.RoomID,
	}
	if groupRoomName != "" {
		response["group_room_name"] = groupRoomName
	}
	return response
}

// getStudentInGroupRoom checks if a student is in their educational group's room
func (rs *Resource) getStudentInGroupRoom(w http.ResponseWriter, r *http.Request) {
	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Check if student has an educational group
	if student.GroupID == nil {
		common.Respond(w, r, http.StatusOK, map[string]interface{}{
			"in_group_room": false,
			"reason":        "no_group",
		}, "Student has no educational group")
		return
	}

	// Get the educational group
	group, err := rs.EducationService.GetGroup(r.Context(), *student.GroupID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to get student's group", err))
		return
	}

	// Check authorization - only group supervisors can see this information
	if authErr := rs.checkGroupRoomAccessAuthorization(r, *student.GroupID); authErr != nil {
		renderError(w, r, common.ErrorForbidden(authErr))
		return
	}

	// Check if the educational group has a room assigned
	if group.RoomID == nil {
		common.Respond(w, r, http.StatusOK, map[string]interface{}{
			"in_group_room": false,
			"reason":        "group_no_room",
		}, "Educational group has no assigned room")
		return
	}

	// Get the student's current active visit
	visit, err := rs.ActiveService.GetStudentCurrentVisit(r.Context(), student.ID)
	if err != nil {
		common.Respond(w, r, http.StatusOK, map[string]interface{}{
			"in_group_room": false,
			"reason":        "no_active_visit",
		}, "Student has no active visit")
		return
	}

	// Get the active group to check its room
	activeGroup, err := rs.ActiveService.GetActiveGroup(r.Context(), visit.ActiveGroupID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to get active group", err))
		return
	}

	// Build and return the response
	groupRoomName := ""
	if group.Room != nil {
		groupRoomName = group.Room.Name
	}
	response := buildGroupRoomResponse(activeGroup, *group.RoomID, groupRoomName)
	common.Respond(w, r, http.StatusOK, response, "Student room status retrieved successfully")
}

// getStudentCurrentVisit handles getting a student's current visit
func (rs *Resource) getStudentCurrentVisit(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL (we only need the ID, not the full student)
	studentID, err := common.ParseID(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	// Get current visit
	currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(r.Context(), studentID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	if currentVisit == nil {
		common.Respond(w, r, http.StatusOK, nil, "Student has no current visit")
		return
	}

	common.Respond(w, r, http.StatusOK, currentVisit, "Current visit retrieved successfully")
}

// getStudentVisitHistory handles getting a student's visit history for today.
//
// Deprecated: Use GET /students/{id}/attendance-history instead, which supports
// arbitrary date ranges and includes the daily attendance log. This endpoint
// will be removed in a future release.
func (rs *Resource) getStudentVisitHistory(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := common.ParseID(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	// Signal deprecation to clients per RFC 8594.
	w.Header().Set("Deprecation", "true")
	w.Header().Set("Link", `</api/students/{id}/attendance-history>; rel="successor-version"`)
	rs.Logger.Warn("deprecated endpoint called",
		slog.String("endpoint", "GET /students/{id}/visit-history"),
		slog.String("successor", "GET /students/{id}/attendance-history"),
		slog.Int64("student_id", studentID),
	)

	// Get all visits for this student
	visits, err := rs.ActiveService.FindVisitsByStudentID(r.Context(), studentID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Filter to today's visits only
	today := timezone.Today()
	tomorrow := today.Add(24 * time.Hour)

	var todaysVisits []*active.Visit
	for _, visit := range visits {
		if visit.EntryTime.After(today) && visit.EntryTime.Before(tomorrow) {
			todaysVisits = append(todaysVisits, visit)
		}
	}

	common.Respond(w, r, http.StatusOK, todaysVisits, "Visit history retrieved successfully")
}
