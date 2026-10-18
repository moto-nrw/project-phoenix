package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

// Directory is the composed kiosk roster contract.
type Directory = devicescan.Directory

// NewDirectory binds the kiosk roster to the retained people adapter. It
// accepts the existing service, never constructs a legacy factory graph.
func NewDirectory(persons PersonDirectory, activities devicescan.ActivityDirectory, logger *slog.Logger) devicescan.Directory {
	if persons == nil {
		panic("kiosk directory composition: users are required")
	}
	var catalog application.DirectoryActivities
	if activities != nil {
		catalog = activities
	}
	return application.NewDirectory(directoryPeople{persons}, catalog, principals{}, logger)
}

type directoryPeople struct{ PersonDirectory }

func (d directoryPeople) Teachers(ctx context.Context) ([]application.DirectoryTeacher, error) {
	teachers, err := d.ListTeachersWithStaffAndPerson(ctx)
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
