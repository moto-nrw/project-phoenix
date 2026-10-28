package devicescan

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PersonLookup supplies card and person facts to scan workflows.
type PersonLookup interface {
	IsPersonNotFound(error) bool
	FindPersonByID(context.Context, int64) (*ScanPerson, error)
	FindByTagID(context.Context, string) (*ScanPerson, error)
}

// TagCommands binds bracelet assignment to the owning directory.
type TagCommands interface {
	NormalizeTag(string) string
	LinkToRFIDCard(context.Context, int64, string) error
	UnlinkFromRFIDCard(context.Context, int64) error
}

// StaffLookup supplies staff and teacher facts without student capabilities.
type StaffLookup interface {
	GetStaffByID(context.Context, int64) (*ScanStaff, error)
	GetStaffByPersonID(context.Context, int64) (*ScanStaff, error)
	GetStaffWithPersonByIDs(context.Context, []int64) (map[int64]*ScanStaff, error)
	GetTeacherByStaffID(context.Context, int64) (*ScanTeacher, error)
	ListTeachersWithStaffAndPerson(context.Context) ([]*ScanTeacher, error)
}

// StudentLookup supplies the scan student's state and kiosk roster.
type StudentLookup interface {
	GetStudentByIDForUpdate(context.Context, int64) (*ScanStudent, error)
	GetStudentByPersonID(context.Context, int64) (*ScanStudent, error)
	Students(context.Context, []int64) ([]DirectoryStudent, error)
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
