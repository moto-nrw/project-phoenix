package schoolstructurehttp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// The children of a group and where they are: the group's student list and
// room status, read through the People Directory port and the presence
// facades.

// GroupStudentResponse represents a student in a group response
type GroupStudentResponse struct {
	ID          int64  `json:"id"`
	PersonID    int64  `json:"person_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
	GroupID     int64  `json:"group_id"`
	GroupName   string `json:"group_name"`
	Location    string `json:"location,omitempty"`
	TagID       string `json:"tag_id,omitempty"`
}

// getStudentCount returns the number of students in a group.
func (rs *Resource) getStudentCount(ctx context.Context, groupID int64) int {
	students, err := rs.People.GroupStudents(ctx, groupID)
	if err != nil {
		return 0
	}
	return len(students)
}

// buildStudentResponse creates a student response with all necessary data
func (rs *Resource) buildStudentResponse(
	ctx context.Context,
	student peopledirectory.StudentRecord,
	group *education.Group,
	hasFullAccess bool,
	locationSnapshot *common.StudentLocationSnapshot,
) *GroupStudentResponse {
	person, err := rs.People.FindPerson(ctx, student.PersonID)
	if err != nil {
		slog.Default().Error("failed to get person data for student",
			slog.Int64("student_id", student.ID),
			slog.String("error", err.Error()))
		return nil
	}

	response := &GroupStudentResponse{
		ID:          student.ID,
		PersonID:    student.PersonID,
		FirstName:   person.FirstName,
		LastName:    person.LastName,
		SchoolClass: student.SchoolClass,
		GroupID:     group.ID,
		GroupName:   group.Name,
	}

	if hasFullAccess && person.TagID != nil {
		response.TagID = *person.TagID
	}
	response.Location = rs.resolveLocationForStudent(ctx, student.ID, hasFullAccess, locationSnapshot)

	return response
}

// resolveLocationForStudent determines student location from snapshot or fallback
func (rs *Resource) resolveLocationForStudent(
	ctx context.Context,
	studentID int64,
	hasFullAccess bool,
	snapshot *common.StudentLocationSnapshot,
) string {
	if snapshot != nil {
		return snapshot.ResolveStudentLocation(studentID, hasFullAccess)
	}
	return rs.resolveStudentLocation(ctx, studentID, hasFullAccess)
}

// getStudentVisitGroupID reads the student's current group ID from the snapshot or service.
func (rs *Resource) getStudentVisitGroupID(ctx context.Context, studentID int64, snapshot *common.StudentLocationSnapshot) *int64 {
	if snapshot != nil {
		if visit := snapshot.Visits[studentID]; visit != nil {
			return &visit.ActiveGroupID
		}
		return nil
	}
	visit, err := rs.ActiveService.GetStudentCurrentVisit(ctx, studentID)
	if err != nil || visit == nil {
		return nil
	}
	return &visit.ActiveGroupID
}

// getVisitActiveGroup retrieves the active group for a visit from snapshot or service
func (rs *Resource) getVisitActiveGroup(ctx context.Context, activeGroupID int64, snapshot *common.StudentLocationSnapshot) *studentpresence.SessionDetail {
	if snapshot != nil {
		return snapshot.Groups[activeGroupID]
	}
	group, err := rs.ActiveService.GetActiveGroup(ctx, activeGroupID)
	if err != nil {
		return nil
	}
	return group
}

// buildStudentRoomStatus creates the room status map for a single student
func (rs *Resource) buildStudentRoomStatus(
	ctx context.Context,
	student peopledirectory.StudentRecord,
	groupRoomID int64,
	snapshot *common.StudentLocationSnapshot,
	personMap map[int64]peopledirectory.Person,
) map[string]interface{} {
	status := map[string]interface{}{
		"in_group_room": false,
		"reason":        "no_active_visit",
	}

	visitGroupID := rs.getStudentVisitGroupID(ctx, student.ID, snapshot)
	if visitGroupID == nil {
		if person, ok := personMap[student.PersonID]; ok {
			status["first_name"] = person.FirstName
			status["last_name"] = person.LastName
		}
		return status
	}

	activeGroup := rs.getVisitActiveGroup(ctx, *visitGroupID, snapshot)
	if activeGroup == nil {
		if person, ok := personMap[student.PersonID]; ok {
			status["first_name"] = person.FirstName
			status["last_name"] = person.LastName
		}
		return status
	}

	inGroupRoom := activeGroup.RoomID == groupRoomID
	status["in_group_room"] = inGroupRoom
	status["current_room_id"] = activeGroup.RoomID

	if inGroupRoom {
		delete(status, "reason")
	} else {
		status["reason"] = "in_different_room"
	}

	if person, ok := personMap[student.PersonID]; ok {
		status["first_name"] = person.FirstName
		status["last_name"] = person.LastName
	}
	return status
}

// buildNoRoomResponse creates the response when group has no room assigned
func buildNoRoomResponse(students []peopledirectory.StudentRecord) map[string]interface{} {
	result := map[string]interface{}{
		"group_has_room":      false,
		"student_room_status": make(map[string]interface{}),
	}

	statusMap := result["student_room_status"].(map[string]interface{})
	for _, student := range students {
		statusMap[strconv.FormatInt(student.ID, 10)] = map[string]interface{}{
			"in_group_room": false,
			"reason":        "group_no_room",
		}
	}
	return result
}

// getGroupStudents gets all students in a specific group
func (rs *Resource) getGroupStudents(w http.ResponseWriter, r *http.Request) {
	// Parse and get group
	group, ok := rs.parseAndGetGroup(w, r)
	if !ok {
		return
	}
	id := group.ID

	// Determine if user can see full student details (admin or group supervisor)
	canAccessFullDetails := rs.userHasGroupAccess(r, id)

	// Get students for this group
	students, err := rs.People.GroupStudents(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	studentIDs := make([]int64, 0, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
	}

	locationSnapshot, snapshotErr := common.LoadStudentLocationSnapshot(r.Context(), rs.ActiveService, studentIDs)
	if snapshotErr != nil {
		slog.Default().Warn("failed to batch load group student locations",
			slog.String("error", snapshotErr.Error()))
		locationSnapshot = nil
	}

	// Build response with person data for each student
	responses := make([]GroupStudentResponse, 0, len(students))
	for _, student := range students {
		response := rs.buildStudentResponse(r.Context(), student, group, canAccessFullDetails, locationSnapshot)
		if response != nil {
			responses = append(responses, *response)
		}
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Found %d students in group", len(responses)))
}

// getGroupStudentsRoomStatus handles getting room status for all students in a group
func (rs *Resource) getGroupStudentsRoomStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidGroupID)
	if !ok {
		return
	}

	group, err := rs.EducationService.GetGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New(common.MsgGroupNotFound)))
		return
	}

	if !rs.userHasGroupAccess(r, id) {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("you do not supervise this group")))
		return
	}

	students, err := rs.People.GroupStudents(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to get group students", err))
		return
	}

	// Handle case where group has no room assigned
	if group.RoomID == nil {
		common.Respond(w, r, http.StatusOK, buildNoRoomResponse(students), "Group has no assigned room")
		return
	}

	// Build room status for each student
	result := rs.buildRoomStatusResponse(r.Context(), students, *group.RoomID)
	common.Respond(w, r, http.StatusOK, result, "Student room status retrieved successfully")
}

// buildRoomStatusResponse creates the full response for room status with student details
func (rs *Resource) buildRoomStatusResponse(ctx context.Context, students []peopledirectory.StudentRecord, groupRoomID int64) map[string]interface{} {
	result := map[string]interface{}{
		"group_has_room": true,
		"group_room_id":  groupRoomID,
	}

	studentIDs := make([]int64, 0, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
	}

	snapshot, snapshotErr := common.LoadStudentLocationSnapshot(ctx, rs.ActiveService, studentIDs)
	if snapshotErr != nil {
		slog.Default().Warn("failed to batch load student room locations",
			slog.String("error", snapshotErr.Error()))
		snapshot = nil
	}

	// Batch-load all persons to avoid N+1 queries
	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
	}
	personMap, personErr := rs.People.ListPersonsByID(ctx, personIDs)
	if personErr != nil {
		slog.Default().Warn("failed to batch load persons",
			slog.String("error", personErr.Error()))
		personMap = make(map[int64]peopledirectory.Person)
	}

	studentStatuses := make(map[string]interface{})
	for _, student := range students {
		studentStatuses[strconv.FormatInt(student.ID, 10)] = rs.buildStudentRoomStatus(ctx, student, groupRoomID, snapshot, personMap)
	}

	result["student_room_status"] = studentStatuses
	return result
}

// resolveStudentLocation determines the student's location string based on active attendance data.
func (rs *Resource) resolveStudentLocation(ctx context.Context, studentID int64, hasFullAccess bool) string {
	attendanceStatus, err := rs.ActiveService.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil || attendanceStatus == nil {
		return "Abwesend"
	}

	if attendanceStatus.Status != "checked_in" {
		return "Abwesend"
	}

	if !hasFullAccess {
		return "Anwesend"
	}

	currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(ctx, studentID)
	if err != nil || currentVisit == nil {
		return "Anwesend"
	}

	if currentVisit.ActiveGroupID <= 0 {
		return "Anwesend"
	}

	activeGroup, err := rs.ActiveService.GetActiveGroup(ctx, currentVisit.ActiveGroupID)
	if err != nil || activeGroup == nil {
		return "Anwesend"
	}

	if activeGroup.Room != nil && activeGroup.Room.Name != "" {
		return fmt.Sprintf("Anwesend - %s", activeGroup.Room.Name)
	}

	return "Anwesend"
}
