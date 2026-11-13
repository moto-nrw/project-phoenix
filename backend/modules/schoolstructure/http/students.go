package schoolstructurehttp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	education "github.com/moto-ogs/project-phoenix/modules/schoolstructure/contract"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
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

// buildStudentResponse creates a student response with all necessary data
func (rs *Resource) buildStudentResponse(
	ctx context.Context,
	student peopledirectory.StudentRecord,
	persons map[int64]peopledirectory.Person,
	group *education.Group,
	hasFullAccess bool,
	locationSnapshot *studentpresence.StudentLocationSnapshot,
) *GroupStudentResponse {
	person, found := persons[student.PersonID]
	var err error
	// Keep the account lookup's error semantics for linked identities. Normal
	// child identities need no account enrichment and use the owner-scoped
	// batch directly.
	if person.AccountID != nil {
		person, err = rs.People.FindPerson(ctx, student.PersonID)
	} else if !found {
		err = fmt.Errorf("person %d: %w", student.PersonID, peopledirectory.ErrPersonNotFound)
	}
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
	response.Location = locationSnapshot.ResolveStudentLocation(student.ID, hasFullAccess)

	return response
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
		common.RenderError(w, r, common.ErrorInternalServerWrap("load group students", err))
		return
	}

	studentIDs := make([]int64, 0, len(students))
	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
		personIDs = append(personIDs, student.PersonID)
	}

	// A failed presence read leaves every location unknown; answering
	// "Abwesend" would report an unconfirmed absence (#6849).
	locationSnapshot, err := studentpresence.LoadStudentLocationSnapshot(r.Context(), rs.ActiveService, studentIDs)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load group student locations", err))
		return
	}

	// An unknown person is absent from the batch, so a failed batch is a
	// failed read (an aborted transaction, say): per-person reads would fail
	// the same way and leave an empty group (#6849).
	persons, err := rs.People.ListPersonsByID(r.Context(), personIDs)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load group student persons", err))
		return
	}

	// Build response with person data for each student
	responses := make([]GroupStudentResponse, 0, len(students))
	for _, student := range students {
		response := rs.buildStudentResponse(r.Context(), student, persons, group, canAccessFullDetails, locationSnapshot)
		if response != nil {
			responses = append(responses, *response)
		}
	}
	if err := rs.endInClassAfterCareDay(r.Context(), responses); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Found %d students in group", len(responses)))
}
