package api

import (
	"context"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	peopleRouteValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	presenceprojectionOwnerHTTP "github.com/moto-nrw/project-phoenix/modules/presenceprojection/http"

	educationSvc "github.com/moto-nrw/project-phoenix/modules/schoolstructure"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The students routes declare their own ports to School Structure and
// Timetable & Activities (#3356); this root binds them to the owners' group
// and enrollment services, so the HTTP resource names neither owner's types.

// studentSchoolGroups serves the students' SchoolGroups port from the
// School Structure group service. The group reads and the group teachers
// below shadow the embedded service's own and reduce each to the students'
// view.
type studentSchoolGroups struct {
	educationSvc.GroupManagement
}

// GetGroupTeachers reduces the group's teachers to the supervisor contact the
// detail lists; a teacher without a staff member or person is left out.
func (s studentSchoolGroups) GetGroupTeachers(ctx context.Context, groupID int64) ([]presenceprojectionOwnerHTTP.GroupTeacher, error) {
	teachers, err := s.GroupManagement.GetGroupTeachers(ctx, groupID)
	if err != nil {
		return nil, err
	}
	result := make([]presenceprojectionOwnerHTTP.GroupTeacher, 0, len(teachers))
	for _, teacher := range teachers {
		if teacher == nil || teacher.Person == nil {
			continue
		}
		result = append(result, presenceprojectionOwnerHTTP.GroupTeacher{
			ID:        teacher.ID,
			FirstName: teacher.Person.FirstName,
			LastName:  teacher.Person.LastName,
			Email:     teacher.Person.Email,
		})
	}
	return result, nil
}

func (s studentSchoolGroups) GetGroup(ctx context.Context, id int64) (*presenceprojectionOwnerHTTP.SchoolGroup, error) {
	group, err := s.GroupManagement.GetGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	view := presenceprojectionOwnerHTTP.SchoolGroup{ID: group.ID, Name: group.Name, RoomID: group.RoomID}
	if group.Room != nil {
		view.RoomName = group.Room.Name
	}
	return &view, nil
}

func (s studentSchoolGroups) GetGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*presenceprojectionOwnerHTTP.SchoolGroup, error) {
	groups, err := s.GroupManagement.GetGroupsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	views := make(map[int64]*presenceprojectionOwnerHTTP.SchoolGroup, len(groups))
	for id, group := range groups {
		if group == nil {
			continue
		}
		view := presenceprojectionOwnerHTTP.SchoolGroup{ID: group.ID, Name: group.Name, RoomID: group.RoomID}
		if group.Room != nil {
			view.RoomName = group.Room.Name
		}
		views[id] = &view
	}
	return views, nil
}

func (s studentSchoolGroups) ListGroups(ctx context.Context) ([]*presenceprojectionOwnerHTTP.SchoolGroup, error) {
	groups, err := s.GroupManagement.ListGroups(ctx, nil)
	if err != nil {
		return nil, err
	}
	views := make([]*presenceprojectionOwnerHTTP.SchoolGroup, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		view := presenceprojectionOwnerHTTP.SchoolGroup{ID: group.ID, Name: group.Name, RoomID: group.RoomID}
		if group.Room != nil {
			view.RoomName = group.Room.Name
		}
		views = append(views, &view)
	}
	return views, nil
}

// studentActiveEnrollments serves the students' ActiveEnrollments port from
// the Timetable enrollment read, reduced to the group id and name the export
// prints. Each child's groups keep the read's order; the export sorts and
// deduplicates the names itself.
type studentActiveEnrollments struct {
	enrollments activityContract.ActivityService
}

func (s studentActiveEnrollments) ActiveEnrollmentGroups(ctx context.Context, studentIDs []int64, onDate calendar.Date) (map[int64][]peopleRouteValues.StudentActivityGroup, error) {
	byStudent, err := s.enrollments.GetActiveStudentEnrollmentsByStudentIDs(ctx, studentIDs, onDate)
	if err != nil {
		return nil, err
	}
	views := make(map[int64][]peopleRouteValues.StudentActivityGroup, len(byStudent))
	for studentID, groups := range byStudent {
		for _, group := range groups {
			if group == nil {
				continue
			}
			views[studentID] = append(views[studentID], peopleRouteValues.StudentActivityGroup{ID: group.ID, Name: group.Name})
		}
	}
	return views, nil
}
