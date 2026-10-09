package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
)

// Directory is the composed kiosk roster contract.
type Directory = devicescan.Directory

// NewDirectory binds the kiosk roster to the retained people adapter. It
// accepts the existing service, never constructs a legacy factory graph.
func NewDirectory(persons PersonDirectory, activities activitiesSvc.ActivityService, logger *slog.Logger) devicescan.Directory {
	if persons == nil {
		panic("kiosk directory composition: users are required")
	}
	var catalog application.DirectoryActivities
	if activities != nil {
		catalog = activityCatalog{activities: activities}
	}
	return application.NewDirectory(directoryPeople{users: persons}, catalog, principals{}, logger)
}

func (c activityCatalog) Activities(ctx context.Context) ([]devicescan.DirectoryActivity, error) {
	groups, err := c.activities.ListGroupsWithOccupancy(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]devicescan.DirectoryActivity, 0, len(groups))
	for _, group := range groups {
		category := ""
		if group.Category != nil {
			category = group.Category.Name
		}
		result = append(result, devicescan.DirectoryActivity{
			ID: group.ID, Name: group.Name, Category: category, IsOccupied: group.IsOccupied,
		})
	}
	return result, nil
}

type directoryPeople struct{ users PersonDirectory }

func (d directoryPeople) Teachers(ctx context.Context) ([]application.DirectoryTeacher, error) {
	teachers, err := d.users.ListTeachersWithStaffAndPerson(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]application.DirectoryTeacher, 0, len(teachers))
	for _, teacher := range teachers {
		row := application.DirectoryTeacher{}
		if teacher != nil {
			row.StaffID = teacher.StaffID
			if teacher.Staff != nil && teacher.Staff.Person != nil {
				person := teacher.Staff.Person
				row.Person = &ports.Person{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName}
			}
		}
		result = append(result, row)
	}
	return result, nil
}

func (d directoryPeople) Students(ctx context.Context, teacherIDs []int64) ([]devicescan.DirectoryStudent, error) {
	var students []users.StudentWithGroupInfo
	var err error
	if teacherIDs == nil {
		students, err = d.users.GetAllStudentsWithGroups(ctx)
	} else {
		students, err = d.users.GetStudentsWithGroupsByTeacherStaffIDs(ctx, teacherIDs)
	}
	if err != nil {
		return nil, err
	}
	result := make([]devicescan.DirectoryStudent, 0, len(students))
	for _, row := range students {
		if row.Student == nil || row.Person == nil {
			continue
		}
		student := row.Student
		tag := ""
		if student.Person.TagID != nil {
			tag = *student.Person.TagID
		}
		result = append(result, devicescan.DirectoryStudent{
			StudentID: student.ID, PersonID: student.PersonID,
			FirstName: student.Person.FirstName, LastName: student.Person.LastName,
			SchoolClass: student.SchoolClass, GroupName: row.GroupName, RFIDTag: tag,
		})
	}
	return result, nil
}
