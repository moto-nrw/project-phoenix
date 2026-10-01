package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
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
	educationSvc.Service
}

// GetGroupTeachers reduces the group's teachers to the supervisor contact the
// detail lists; a teacher without a staff member or person is left out.
func (s studentSchoolGroups) GetGroupTeachers(ctx context.Context, groupID int64) ([]students.GroupTeacher, error) {
	teachers, err := s.Service.GetGroupTeachers(ctx, groupID)
	if err != nil {
		return nil, err
	}
	result := make([]students.GroupTeacher, 0, len(teachers))
	for _, teacher := range teachers {
		if teacher == nil || teacher.Staff == nil || teacher.Staff.Person == nil {
			continue
		}
		view := students.GroupTeacher{
			ID:        teacher.ID,
			FirstName: teacher.Staff.Person.FirstName,
			LastName:  teacher.Staff.Person.LastName,
		}
		if teacher.Staff.Person.Account != nil {
			view.Email = teacher.Staff.Person.Account.Email
		}
		result = append(result, view)
	}
	return result, nil
}

func (s studentSchoolGroups) GetGroup(ctx context.Context, id int64) (*students.SchoolGroup, error) {
	group, err := s.Service.GetGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	view := students.SchoolGroup{ID: group.ID, Name: group.Name, RoomID: group.RoomID}
	if group.Room != nil {
		view.RoomName = group.Room.Name
	}
	return &view, nil
}

func (s studentSchoolGroups) GetGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*students.SchoolGroup, error) {
	groups, err := s.Service.GetGroupsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	views := make(map[int64]*students.SchoolGroup, len(groups))
	for id, group := range groups {
		if group == nil {
			continue
		}
		view := students.SchoolGroup{ID: group.ID, Name: group.Name, RoomID: group.RoomID}
		if group.Room != nil {
			view.RoomName = group.Room.Name
		}
		views[id] = &view
	}
	return views, nil
}

func (s studentSchoolGroups) ListGroups(ctx context.Context) ([]*students.SchoolGroup, error) {
	groups, err := s.Service.ListGroups(ctx, nil)
	if err != nil {
		return nil, err
	}
	views := make([]*students.SchoolGroup, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		view := students.SchoolGroup{ID: group.ID, Name: group.Name, RoomID: group.RoomID}
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
	enrollments timetableCompose.ActivityService
}

func (s studentActiveEnrollments) ActiveEnrollmentGroups(ctx context.Context, studentIDs []int64, onDate calendar.Date) (map[int64][]students.ActiveEnrollmentGroup, error) {
	byStudent, err := s.enrollments.GetActiveStudentEnrollmentsByStudentIDs(ctx, studentIDs, onDate)
	if err != nil {
		return nil, err
	}
	views := make(map[int64][]students.ActiveEnrollmentGroup, len(byStudent))
	for studentID, groups := range byStudent {
		for _, group := range groups {
			if group == nil {
				continue
			}
			views[studentID] = append(views[studentID], students.ActiveEnrollmentGroup{ID: group.ID, Name: group.Name})
		}
	}
	return views, nil
}
