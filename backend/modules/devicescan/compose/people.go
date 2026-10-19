package compose

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"

	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PersonDirectory is the People Directory slice the kiosk flows use: card
// lookup and assignment, the kiosk roster and the records behind a scan,
// which the root maps from the retained people service.
type PersonDirectory interface {
	IsPersonNotFound(error) bool
	NormalizeTag(tag string) string
	Get(ctx context.Context, id interface{}) (*ScanPerson, error)
	FindByTagID(ctx context.Context, tagID string) (*ScanPerson, error)
	LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error
	UnlinkFromRFIDCard(ctx context.Context, personID int64) error
	GetStaffByID(ctx context.Context, id int64) (*ScanStaff, error)
	GetStaffByPersonID(ctx context.Context, personID int64) (*ScanStaff, error)
	GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*ScanStaff, error)
	GetTeacherByStaffID(ctx context.Context, staffID int64) (*ScanTeacher, error)
	ListTeachersWithStaffAndPerson(ctx context.Context) ([]*ScanTeacher, error)
	GetStudentByIDForUpdate(ctx context.Context, id int64) (*ScanStudent, error)
	GetStudentByPersonID(ctx context.Context, personID int64) (*ScanStudent, error)
	// Students lists the kiosk roster: every child, or those of teacherIDs.
	Students(ctx context.Context, teacherIDs []int64) ([]devicescan.DirectoryStudent, error)
}

type (
	ScanPerson struct {
		ID                  int64
		FirstName, LastName string
		TagID               *string
	}
	ScanStaff struct {
		ID, PersonID int64
		Person       *ScanPerson
	}
	ScanTeacher struct {
		StaffID              int64
		Role, Specialization string
		Staff                *ScanStaff
	}
	ScanStudent struct {
		ID, PersonID  int64
		GroupID       *int64
		SchoolClass   string
		Alumnus       bool
		EnrolledUntil *calendar.Date
		Person        *ScanPerson
	}
)

// people resolves cards through the retained people service. Its not-found
// outcomes become the port sentinels; every other failure passes through
// unchanged so the flows keep their own classification.
type people struct{ PersonDirectory }

func (p people) FindPersonByTag(ctx context.Context, tag string) (*ports.Person, error) {
	person, err := p.FindByTagID(ctx, tag)
	if err != nil {
		if p.IsPersonNotFound(err) {
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
	student, err := p.GetStudentByPersonID(ctx, personID)
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
		Alumnus:       student.Alumnus,
		EnrolledUntil: student.EnrolledUntil,
	}, nil
}

func (p people) FindStaffByPerson(ctx context.Context, personID int64) (*ports.StaffMember, error) {
	staff, err := p.GetStaffByPersonID(ctx, personID)
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
