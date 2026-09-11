package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

// TagPeople supplies the person and membership facts needed to distinguish
// a free bracelet from one the kiosk can actually assign or release.
type TagPeople interface {
	ports.People
	TeacherRole(context.Context, int64) (role string, specialization string, found bool, err error)
	FindStaff(context.Context, int64) (*ports.StaffMember, error)
	FindPerson(context.Context, int64) (*ports.Person, error)
	LinkTag(context.Context, int64, string) error
	UnlinkTag(context.Context, int64) error
}

type tagAssignments struct {
	people     TagPeople
	principals ports.Principals
	clock      ports.Clock
	logger     *slog.Logger
}

func NewTagAssignments(people TagPeople, principals ports.Principals, clock ports.Clock, logger *slog.Logger) devicescan.TagAssignments {
	if people == nil || principals == nil || clock == nil {
		panic("tag assignment query: people, principals and clock are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &tagAssignments{people: people, principals: principals, clock: clock, logger: logger}
}

func (q *tagAssignments) LookupTagAssignment(ctx context.Context, tag string) (devicescan.TagAssignment, error) {
	device, ok := q.principals.Device(ctx)
	if !ok || device == nil {
		return devicescan.TagAssignment{}, devicescan.ErrDeviceUnauthorized
	}
	normalized := q.people.NormalizeTag(tag)
	person, err := q.people.FindPersonByTag(ctx, normalized)
	if err != nil {
		if errors.Is(err, ports.ErrPersonNotFound) {
			return devicescan.TagAssignment{Assigned: false}, nil
		}
		q.logger.ErrorContext(ctx, "failed to check RFID assignment", slog.String("error", err.Error()))
		return devicescan.TagAssignment{}, devicescan.Internal("Internal server error", err)
	}
	if person == nil || person.TagID == nil || *person.TagID != normalized {
		return devicescan.TagAssignment{Assigned: false}, nil
	}
	name := person.FirstName + " " + person.LastName
	if assigned := q.student(ctx, person, name); assigned != nil {
		return *assigned, nil
	}
	if assigned := q.staff(ctx, person, name); assigned != nil {
		return *assigned, nil
	}
	return devicescan.TagAssignment{Assigned: false}, nil
}

func (q *tagAssignments) student(ctx context.Context, person *ports.Person, name string) *devicescan.TagAssignment {
	student, err := q.people.FindStudentByPerson(ctx, person.ID)
	if err != nil || student == nil {
		if err != nil {
			q.logger.WarnContext(ctx, "error finding student for person",
				slog.Int64("person_id", person.ID),
				slog.String("error", err.Error()),
			)
		}
		return nil
	}
	if student.Alumnus {
		q.logger.InfoContext(ctx, "rfid tag still bound to a graduated student, reporting as unassigned", slog.Int64("student_id", student.ID))
		return nil
	}
	if student.EnrolledUntil != nil && q.clock.Day(q.clock.Now()).After(*student.EnrolledUntil) {
		q.logger.InfoContext(ctx, "rfid tag still bound to a departed student, reporting as unassigned", slog.Int64("student_id", student.ID))
		return nil
	}
	return &devicescan.TagAssignment{
		Assigned: true, PersonType: "student",
		Person:  &devicescan.TagAssignedPerson{ID: student.ID, PersonID: person.ID, Name: name, Group: student.SchoolClass},
		Student: &devicescan.TagAssignedStudent{ID: student.ID, Name: name, Group: student.SchoolClass},
	}
}

func (q *tagAssignments) staff(ctx context.Context, person *ports.Person, name string) *devicescan.TagAssignment {
	staff, err := q.people.FindStaffByPerson(ctx, person.ID)
	if err != nil || staff == nil {
		if err != nil {
			q.logger.WarnContext(ctx, "error finding staff for person",
				slog.Int64("person_id", person.ID),
				slog.String("error", err.Error()),
			)
		}
		return nil
	}
	group := "Staff"
	role, specialization, found, err := q.people.TeacherRole(ctx, staff.ID)
	if err != nil {
		q.logger.WarnContext(ctx, "error checking teacher status for staff",
			slog.Int64("staff_id", staff.ID),
			slog.String("error", err.Error()),
		)
	} else if found {
		switch {
		case role != "":
			group = role
		case specialization != "":
			group = specialization
		default:
			group = "Teacher"
		}
	}
	return &devicescan.TagAssignment{Assigned: true, PersonType: "staff", Person: &devicescan.TagAssignedPerson{ID: staff.ID, PersonID: person.ID, Name: name, Group: group}}
}
