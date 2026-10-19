package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// deviceScanUserRows is the slice of the retained people service the kiosk
// flows read.
type deviceScanUserRows interface {
	Get(ctx context.Context, id interface{}) (*peopledirectory.DirectoryPersonSnapshot, error)
	FindByTagID(ctx context.Context, tagID string) (*peopledirectory.DirectoryPersonSnapshot, error)
	LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error
	UnlinkFromRFIDCard(ctx context.Context, personID int64) error
	GetStaffByID(ctx context.Context, id int64) (*peopledirectory.Staff, error)
	GetStaffByPersonID(ctx context.Context, personID int64) (*peopledirectory.Staff, error)
	GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*peopledirectory.Staff, error)
	GetTeacherByStaffID(ctx context.Context, staffID int64) (*peopledirectory.Teacher, error)
	ListTeachersWithStaffAndPerson(ctx context.Context) ([]*peopledirectory.Teacher, error)
	GetStudentByIDForUpdate(ctx context.Context, id int64) (*peopledirectory.StudentState, error)
	GetStudentByPersonID(ctx context.Context, personID int64) (*peopledirectory.StudentState, error)
	GetStudentsWithGroupsByTeacherStaffIDs(ctx context.Context, staffIDs []int64) ([]peopledirectory.StudentRosterEntry, error)
	GetAllStudentsWithGroups(ctx context.Context) ([]peopledirectory.StudentRosterEntry, error)
}

// DeviceScanPeople maps the retained people service onto the device scans'
// records.
type DeviceScanPeople struct{ Users deviceScanUserRows }

// NewDeviceScanPeople binds the scans to the people service; nil leaves them
// unbound.
func NewDeviceScanPeople(users deviceScanUserRows) devicescanCompose.PersonDirectory {
	if users == nil {
		return nil
	}
	return DeviceScanPeople{Users: users}
}

func (DeviceScanPeople) NormalizeTag(tag string) string { return peopledirectory.NormalizeTagID(tag) }

func (p DeviceScanPeople) Get(ctx context.Context, id interface{}) (*devicescanCompose.ScanPerson, error) {
	person, err := p.Users.Get(ctx, id)
	return scanPerson(person), err
}

func (p DeviceScanPeople) FindByTagID(ctx context.Context, tagID string) (*devicescanCompose.ScanPerson, error) {
	person, err := p.Users.FindByTagID(ctx, tagID)
	return scanPerson(person), err
}

func (p DeviceScanPeople) LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	return p.Users.LinkToRFIDCard(ctx, personID, tagID)
}

func (p DeviceScanPeople) UnlinkFromRFIDCard(ctx context.Context, personID int64) error {
	return p.Users.UnlinkFromRFIDCard(ctx, personID)
}

func (p DeviceScanPeople) GetStaffByID(ctx context.Context, id int64) (*devicescanCompose.ScanStaff, error) {
	staff, err := p.Users.GetStaffByID(ctx, id)
	return scanStaff(staff), err
}

func (p DeviceScanPeople) GetStaffByPersonID(ctx context.Context, personID int64) (*devicescanCompose.ScanStaff, error) {
	staff, err := p.Users.GetStaffByPersonID(ctx, personID)
	return scanStaff(staff), err
}

func (p DeviceScanPeople) GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*devicescanCompose.ScanStaff, error) {
	rows, err := p.Users.GetStaffWithPersonByIDs(ctx, ids)
	if rows == nil {
		return nil, err
	}
	result := make(map[int64]*devicescanCompose.ScanStaff, len(rows))
	for id, row := range rows {
		result[id] = scanStaff(row)
	}
	return result, err
}

func (p DeviceScanPeople) GetTeacherByStaffID(ctx context.Context, staffID int64) (*devicescanCompose.ScanTeacher, error) {
	teacher, err := p.Users.GetTeacherByStaffID(ctx, staffID)
	return scanTeacher(teacher), err
}

func (p DeviceScanPeople) ListTeachersWithStaffAndPerson(ctx context.Context) ([]*devicescanCompose.ScanTeacher, error) {
	rows, err := p.Users.ListTeachersWithStaffAndPerson(ctx)
	if rows == nil {
		return nil, err
	}
	result := make([]*devicescanCompose.ScanTeacher, len(rows))
	for i, row := range rows {
		result[i] = scanTeacher(row)
	}
	return result, err
}

func (p DeviceScanPeople) GetStudentByIDForUpdate(ctx context.Context, id int64) (*devicescanCompose.ScanStudent, error) {
	student, err := p.Users.GetStudentByIDForUpdate(ctx, id)
	return scanStudent(student), err
}

func (p DeviceScanPeople) GetStudentByPersonID(ctx context.Context, personID int64) (*devicescanCompose.ScanStudent, error) {
	student, err := p.Users.GetStudentByPersonID(ctx, personID)
	return scanStudent(student), err
}

// Students lists the kiosk roster: every child, or those of the teachers'
// groups, with name, class, group and card.
func (p DeviceScanPeople) Students(ctx context.Context, teacherIDs []int64) ([]devicescan.DirectoryStudent, error) {
	var students []peopledirectory.StudentRosterEntry
	var err error
	if teacherIDs == nil {
		students, err = p.Users.GetAllStudentsWithGroups(ctx)
	} else {
		students, err = p.Users.GetStudentsWithGroupsByTeacherStaffIDs(ctx, teacherIDs)
	}
	if err != nil {
		return nil, err
	}
	result := make([]devicescan.DirectoryStudent, 0, len(students))
	for _, row := range students {
		if row.Record == nil || row.Record.Person == nil {
			continue
		}
		student := row.Record
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

func scanPerson(person *peopledirectory.DirectoryPersonSnapshot) *devicescanCompose.ScanPerson {
	if person == nil {
		return nil
	}
	return &devicescanCompose.ScanPerson{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName, TagID: person.TagID}
}

func scanStaff(staff *peopledirectory.Staff) *devicescanCompose.ScanStaff {
	if staff == nil {
		return nil
	}
	return &devicescanCompose.ScanStaff{ID: staff.ID, PersonID: staff.PersonID, Person: scanPerson(staff.Person)}
}

func scanTeacher(teacher *peopledirectory.Teacher) *devicescanCompose.ScanTeacher {
	if teacher == nil {
		return nil
	}
	return &devicescanCompose.ScanTeacher{StaffID: teacher.StaffID, Role: teacher.Role, Specialization: teacher.Specialization, Staff: scanStaff(teacher.Staff)}
}

func scanStudent(student *peopledirectory.StudentState) *devicescanCompose.ScanStudent {
	if student == nil {
		return nil
	}
	return &devicescanCompose.ScanStudent{
		ID: student.ID, PersonID: student.PersonID, GroupID: student.GroupID, SchoolClass: student.SchoolClass,
		Alumnus: student.IsAlumnus(), EnrolledUntil: student.EnrolledUntil,
		Person: scanPerson(student.Person),
	}
}

func (DeviceScanPeople) IsPersonNotFound(err error) bool { return peoplePorts.IsPersonNotFound(err) }
