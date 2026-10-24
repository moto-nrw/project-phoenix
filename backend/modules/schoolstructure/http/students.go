package schoolstructurehttp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"
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
	locationSnapshot *studentpresence.StudentLocationSnapshot,
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
	snapshot *studentpresence.StudentLocationSnapshot,
) string {
	if snapshot != nil {
		return snapshot.ResolveStudentLocation(studentID, hasFullAccess)
	}
	return rs.resolveStudentLocation(ctx, studentID, hasFullAccess)
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

	locationSnapshot, snapshotErr := studentpresence.LoadStudentLocationSnapshot(r.Context(), rs.ActiveService, studentIDs)
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
