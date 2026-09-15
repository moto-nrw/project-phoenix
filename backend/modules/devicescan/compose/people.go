package compose

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// people resolves cards through the retained people service. Its not-found
// outcomes become the port sentinels; every other failure passes through
// unchanged so the flows keep their own classification.
type people struct{ users usersSvc.PersonService }

func (people) NormalizeTag(tag string) string { return users.NormalizeTagID(tag) }

func (p people) FindPersonByTag(ctx context.Context, tag string) (*ports.Person, error) {
	person, err := p.users.FindByTagID(ctx, tag)
	if err != nil {
		if errors.Is(err, usersSvc.ErrPersonNotFound) {
			return nil, mapError(err, ports.ErrPersonNotFound)
		}
		return nil, err
	}
	if person == nil {
		return nil, ports.ErrPersonNotFound
	}
	return &ports.Person{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName, HasTag: person.TagID != nil, TagID: person.TagID}, nil
}

func (p people) FindStudentByPerson(ctx context.Context, personID int64) (*ports.Student, error) {
	student, err := p.users.GetStudentByPersonID(ctx, personID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if student == nil {
		return nil, nil
	}
	return &ports.Student{
		ID: student.ID, PersonID: student.PersonID, GroupID: student.GroupID, SchoolClass: student.SchoolClass,
		Alumnus:       student.Status == users.StudentStatusAlumnus,
		EnrolledUntil: student.EnrolledUntil,
	}, nil
}

func (p people) FindStaffByPerson(ctx context.Context, personID int64) (*ports.StaffMember, error) {
	staff, err := p.users.GetStaffByPersonID(ctx, personID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if staff == nil {
		return nil, nil
	}
	return &ports.StaffMember{ID: staff.ID, PersonID: staff.PersonID}, nil
}
