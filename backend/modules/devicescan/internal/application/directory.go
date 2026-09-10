package application

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

// DirectoryTeacher is a roster assignment with its optional joined person.
type DirectoryTeacher struct {
	StaffID int64
	Person  *ports.Person
}

// DirectoryPeople supplies tenant-scoped roster facts.
type DirectoryPeople interface {
	Teachers(context.Context) ([]DirectoryTeacher, error)
	Students(context.Context, []int64) ([]devicescan.DirectoryStudent, error)
}

type DirectoryActivities interface {
	Activities(context.Context) ([]devicescan.DirectoryActivity, error)
}

type directory struct {
	activities DirectoryActivities
	people     DirectoryPeople
	principals ports.Principals
	logger     *slog.Logger
}

func NewDirectory(people DirectoryPeople, activities DirectoryActivities, principals ports.Principals, logger *slog.Logger) devicescan.Directory {
	if people == nil || principals == nil {
		panic("kiosk directory: people and principals are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &directory{people: people, activities: activities, principals: principals, logger: logger}
}

func (d *directory) Activities(ctx context.Context) ([]devicescan.DirectoryActivity, error) {
	device, ok := d.principals.Device(ctx)
	if !ok || device == nil {
		return nil, devicescan.ErrDeviceUnauthorized
	}
	if d.activities == nil {
		return nil, errors.New("kiosk directory: activities are not configured")
	}
	return d.activities.Activities(ctx)
}

func (d *directory) Teachers(ctx context.Context) ([]devicescan.Teacher, error) {
	device, ok := d.principals.Device(ctx)
	if !ok || device == nil {
		return nil, devicescan.ErrDeviceUnauthorized
	}
	teachers, err := d.people.Teachers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]devicescan.Teacher, 0, len(teachers))
	for _, teacher := range teachers {
		if teacher.Person == nil {
			continue
		}
		person := teacher.Person
		result = append(result, devicescan.Teacher{
			StaffID: teacher.StaffID, PersonID: person.ID,
			FirstName: person.FirstName, LastName: person.LastName,
			DisplayName: strings.TrimSpace(person.FirstName + " " + person.LastName),
		})
	}
	d.logger.InfoContext(ctx, "device requested teacher list",
		slog.String("device_id", device.DeviceID),
		slog.Int("teacher_count", len(result)),
	)
	return result, nil
}

func (d *directory) Students(ctx context.Context, teacherIDs []int64) ([]devicescan.DirectoryStudent, error) {
	device, ok := d.principals.Device(ctx)
	if !ok || device == nil {
		return nil, devicescan.ErrDeviceUnauthorized
	}
	if teacherIDs != nil && len(teacherIDs) == 0 {
		return []devicescan.DirectoryStudent{}, nil
	}
	students, err := d.people.Students(ctx, teacherIDs)
	if err != nil {
		if teacherIDs == nil {
			return nil, err
		}
		// The filtered roster has historically returned an empty result when
		// the batch lookup fails. Preserve that contract during contraction.
		d.logger.WarnContext(ctx, "failed to batch-find students for teachers", slog.String("error", err.Error()))
		return []devicescan.DirectoryStudent{}, nil
	}
	if teacherIDs == nil {
		return students, nil
	}
	unique := make(map[int64]devicescan.DirectoryStudent, len(students))
	for _, student := range students {
		unique[student.StudentID] = student
	}
	result := make([]devicescan.DirectoryStudent, 0, len(unique))
	for _, student := range unique {
		result = append(result, student)
	}
	return result, nil
}
