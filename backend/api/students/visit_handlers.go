package students

import (
	"errors"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
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
	hasFullAccess := rs.checkStudentFullAccess(r, student)

	// Build student response
	response := newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
		Student:       student,
		Person:        person,
		Group:         group,
		HasFullAccess: hasFullAccess,
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

// checkGroupRoomAccessAuthorization verifies if the user can view student room status
// Returns an error if unauthorized, nil if authorized
func (rs *Resource) checkGroupRoomAccessAuthorization(r *http.Request, studentGroupID int64) error {
	if hasAdminPermissions(getPermissionsFromRequest(r)) {
		return nil
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
		renderError(w, r, ErrorInternalServer(errors.New("failed to get student's group")))
		return
	}

	// Check authorization - only group supervisors can see this information
	if authErr := rs.checkGroupRoomAccessAuthorization(r, *student.GroupID); authErr != nil {
		renderError(w, r, ErrorForbidden(authErr))
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
		renderError(w, r, ErrorInternalServer(errors.New("failed to get active group")))
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
		renderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	// Get current visit
	currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(r.Context(), studentID)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	if currentVisit == nil {
		common.Respond(w, r, http.StatusOK, nil, "Student has no current visit")
		return
	}

	common.Respond(w, r, http.StatusOK, currentVisit, "Current visit retrieved successfully")
}

// getStudentVisitHistory handles getting a student's visit history for today
func (rs *Resource) getStudentVisitHistory(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := common.ParseID(r)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	// Get all visits for this student
	visits, err := rs.ActiveService.FindVisitsByStudentID(r.Context(), studentID)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
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
