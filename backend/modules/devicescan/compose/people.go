package compose

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/userscontract"
)

// PersonDirectory is the slice of the retained people service the kiosk flows
// use: card lookup and assignment, the kiosk roster, and the staff, teacher and
// student records behind a scan.
type PersonDirectory interface {
	Get(ctx context.Context, id interface{}) (*users.Person, error)
	FindByTagID(ctx context.Context, tagID string) (*users.Person, error)
	LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error
	UnlinkFromRFIDCard(ctx context.Context, personID int64) error
	GetStaffByID(ctx context.Context, id int64) (*users.Staff, error)
	GetStaffByPersonID(ctx context.Context, personID int64) (*users.Staff, error)
	GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*users.Staff, error)
	GetTeacherByStaffID(ctx context.Context, staffID int64) (*users.Teacher, error)
	ListTeachersWithStaffAndPerson(ctx context.Context) ([]*users.Teacher, error)
	GetStudentByIDForUpdate(ctx context.Context, id int64) (*users.Student, error)
	GetStudentByPersonID(ctx context.Context, personID int64) (*users.Student, error)
	GetStudentsWithGroupsByTeacherStaffIDs(ctx context.Context, staffIDs []int64) ([]users.StudentWithGroupInfo, error)
	GetAllStudentsWithGroups(ctx context.Context) ([]users.StudentWithGroupInfo, error)
}

// people resolves cards through the retained people service. Its not-found
// outcomes become the port sentinels; every other failure passes through
// unchanged so the flows keep their own classification.
type people struct{ users PersonDirectory }

func (people) NormalizeTag(tag string) string { return users.NormalizeTagID(tag) }

func (p people) FindPersonByTag(ctx context.Context, tag string) (*ports.Person, error) {
	person, err := p.users.FindByTagID(ctx, tag)
	if err != nil {
		if errors.Is(err, userscontract.ErrPersonNotFound) {
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
