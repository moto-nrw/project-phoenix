package groups

import (
	"context"
	"fmt"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// =============================================================================
// STUDENT RESPONSE HELPERS
// =============================================================================

// populateGuardianDetails fills in guardian fields based on access permissions
func populateGuardianDetails(response *GroupStudentResponse, student *users.Student, person *users.Person, hasFullAccess bool) {
	// Full access users get all guardian details and tag ID
	if hasFullAccess {
		if student.GuardianName != nil {
			response.GuardianName = *student.GuardianName
		}
		if student.GuardianContact != nil {
			response.GuardianContact = *student.GuardianContact
		}
		if student.GuardianEmail != nil {
			response.GuardianEmail = *student.GuardianEmail
		}
		if student.GuardianPhone != nil {
			response.GuardianPhone = *student.GuardianPhone
		}
		if person.TagID != nil {
			response.TagID = *person.TagID
		}
		return
	}

	// Limited access: only guardian name visible
	if student.GuardianName != nil {
		response.GuardianName = *student.GuardianName
	}
}

// buildStudentResponse creates a student response with all necessary data
func (rs *Resource) buildStudentResponse(
	ctx context.Context,
	student *users.Student,
	group *education.Group,
	hasFullAccess bool,
	locationSnapshot *common.StudentLocationSnapshot,
) *GroupStudentResponse {
	person, err := rs.UserService.Get(ctx, student.PersonID)
	if err != nil {
		logger.Logger.WithError(err).WithField("student_id", student.ID).Warn("Failed to get person data for student")
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

	populateGuardianDetails(response, student, person, hasFullAccess)
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

// resolveStudentLocation determines the student's location string based on active attendance data.
func (rs *Resource) resolveStudentLocation(ctx context.Context, studentID int64, hasFullAccess bool) string {
	attendanceStatus, err := rs.ActiveService.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil || attendanceStatus == nil {
		return "Abwesend"
	}

	if attendanceStatus.Status != activeService.StatusCheckedIn {
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

// =============================================================================
// ROOM STATUS HELPERS
// =============================================================================

// getStudentVisit retrieves a student's current visit from snapshot or service
func (rs *Resource) getStudentVisit(ctx context.Context, studentID int64, snapshot *common.StudentLocationSnapshot) *active.Visit {
	if snapshot != nil {
		return snapshot.Visits[studentID]
	}
	visit, err := rs.ActiveService.GetStudentCurrentVisit(ctx, studentID)
	if err != nil {
		return nil
	}
	return visit
}

// getVisitActiveGroup retrieves the active group for a visit from snapshot or service
func (rs *Resource) getVisitActiveGroup(ctx context.Context, visit *active.Visit, snapshot *common.StudentLocationSnapshot) *active.Group {
	if snapshot != nil {
		return snapshot.Groups[visit.ActiveGroupID]
	}
	group, err := rs.ActiveService.GetActiveGroup(ctx, visit.ActiveGroupID)
	if err != nil {
		return nil
	}
	return group
}

// buildStudentRoomStatus creates the room status map for a single student
func (rs *Resource) buildStudentRoomStatus(
	ctx context.Context,
	student *users.Student,
	groupRoomID int64,
	snapshot *common.StudentLocationSnapshot,
) map[string]interface{} {
	status := map[string]interface{}{
		"in_group_room": false,
		"reason":        "no_active_visit",
	}

	visit := rs.getStudentVisit(ctx, student.ID, snapshot)
	if visit == nil {
		rs.addPersonDataToStatus(ctx, status, student.PersonID)
		return status
	}

	activeGroup := rs.getVisitActiveGroup(ctx, visit, snapshot)
	if activeGroup == nil {
		rs.addPersonDataToStatus(ctx, status, student.PersonID)
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

	rs.addPersonDataToStatus(ctx, status, student.PersonID)
	return status
}

// addPersonDataToStatus adds first_name and last_name to status map
func (rs *Resource) addPersonDataToStatus(ctx context.Context, status map[string]interface{}, personID int64) {
	person, err := rs.UserService.Get(ctx, personID)
	if err == nil && person != nil {
		status["first_name"] = person.FirstName
		status["last_name"] = person.LastName
	}
}

// buildNoRoomResponse creates the response when group has no room assigned
func buildNoRoomResponse(students []*users.Student) map[string]interface{} {
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

// buildRoomStatusResponse creates the full response for room status with student details
func (rs *Resource) buildRoomStatusResponse(ctx context.Context, students []*users.Student, groupRoomID int64) map[string]interface{} {
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
		logger.Logger.WithError(snapshotErr).Warn("Failed to batch load student room locations")
		snapshot = nil
	}

	studentStatuses := make(map[string]interface{})
	for _, student := range students {
		studentStatuses[strconv.FormatInt(student.ID, 10)] = rs.buildStudentRoomStatus(ctx, student, groupRoomID, snapshot)
	}

	result["student_room_status"] = studentStatuses
	return result
}
