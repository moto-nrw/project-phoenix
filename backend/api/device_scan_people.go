package api

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// deviceScanUserRows is the slice of the retained people service the kiosk
// flows read.
type deviceScanUserRows interface {
	Get(ctx context.Context, id interface{}) (*peopledirectory.DirectoryPersonSnapshot, error)
	FindByTagID(ctx context.Context, tagID string) (*peopledirectory.DirectoryPersonSnapshot, error)
	LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error
	UnlinkFromRFIDCard(ctx context.Context, personID int64) error
	StaffRows() peoplePorts.StaffRepository
	TeacherRows() peoplePorts.TeacherRepository
	StudentRows() peoplePorts.StudentRepository
	GetStudentByIDForUpdate(ctx context.Context, id int64) (*peopledirectory.StudentState, error)
	GetStudentsWithGroupsByTeacherStaffIDs(ctx context.Context, staffIDs []int64) ([]peopledirectory.StudentRosterEntry, error)
	GetAllStudentsWithGroups(ctx context.Context) ([]peopledirectory.StudentRosterEntry, error)
}

// kioskOpenInvitations lists the school's unused, unexpired invitations.
type kioskOpenInvitations interface {
	ListPendingSchoolInvitations(ctx context.Context) ([]identityaccess.SchoolInvitation, error)
}

// DeviceScanPeople maps the retained people service onto the device scans'
// records. Invitations decide which external caregivers without an account
// the kiosk teacher list offers (#4573); a binding without them refuses to
// list such a caregiver.
type DeviceScanPeople struct {
	Users       deviceScanUserRows
	Invitations kioskOpenInvitations
}

// NewDeviceScanPeople binds the scans to the people service; nil leaves them
// unbound. It serves every scan but the kiosk teacher list, which
// NewDeviceScanDirectoryPeople binds.
func NewDeviceScanPeople(users deviceScanUserRows) devicescanCompose.PersonDirectory {
	if users == nil {
		return nil
	}
	return DeviceScanPeople{Users: users}
}

// NewDeviceScanDirectoryPeople binds the kiosk roster to the people service
// and the school's open invitations.
func NewDeviceScanDirectoryPeople(users deviceScanUserRows, invitations kioskOpenInvitations) devicescanCompose.PersonDirectory {
	if users == nil {
		return nil
	}
	return DeviceScanPeople{Users: users, Invitations: invitations}
}

func (DeviceScanPeople) NormalizeTag(tag string) string { return peopledirectory.NormalizeTagID(tag) }

func (p DeviceScanPeople) FindPersonByID(ctx context.Context, id int64) (*devicescan.ScanPerson, error) {
	person, err := p.Users.Get(ctx, id)
	return scanPerson(person), err
}

func (p DeviceScanPeople) FindByTagID(ctx context.Context, tagID string) (*devicescan.ScanPerson, error) {
	person, err := p.Users.FindByTagID(ctx, tagID)
	return scanPerson(person), err
}

func (p DeviceScanPeople) LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	return p.Users.LinkToRFIDCard(ctx, personID, tagID)
}

func (p DeviceScanPeople) UnlinkFromRFIDCard(ctx context.Context, personID int64) error {
	return p.Users.UnlinkFromRFIDCard(ctx, personID)
}

func (p DeviceScanPeople) GetStaffByID(ctx context.Context, id int64) (*devicescan.ScanStaff, error) {
	staff, err := p.Users.StaffRows().FindByID(ctx, id)
	return scanStaff(staff), err
}

func (p DeviceScanPeople) GetStaffByPersonID(ctx context.Context, personID int64) (*devicescan.ScanStaff, error) {
	staff, err := p.Users.StaffRows().FindByPersonID(ctx, personID)
	return scanStaff(staff), err
}

func (p DeviceScanPeople) GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*devicescan.ScanStaff, error) {
	rows, err := p.Users.StaffRows().FindWithPersonByIDs(ctx, ids)
	if rows == nil {
		return nil, err
	}
	result := make(map[int64]*devicescan.ScanStaff, len(rows))
	for id, row := range rows {
		result[id] = scanStaff(row)
	}
	return result, err
}

func (p DeviceScanPeople) GetTeacherByStaffID(ctx context.Context, staffID int64) (*devicescan.ScanTeacher, error) {
	teacher, err := p.Users.TeacherRows().FindByStaffID(ctx, staffID)
	return scanTeacher(teacher), err
}

// ListTeachersWithStaffAndPerson lists the caregivers the kiosk offers as
// supervisors. An external caregiver (#3823) stays on the list only while a
// new assignment may name them: with an account or an open invitation
// (School Membership's rule, #4573). Already running supervisions are not
// read from this list.
func (p DeviceScanPeople) ListTeachersWithStaffAndPerson(ctx context.Context) ([]*devicescan.ScanTeacher, error) {
	rows, err := p.Users.TeacherRows().ListAllWithStaffAndPerson(ctx)
	if err != nil || rows == nil {
		return nil, err
	}
	invited, err := p.invitedExternals(ctx, rows)
	if err != nil {
		return nil, err
	}
	result := make([]*devicescan.ScanTeacher, 0, len(rows))
	for _, row := range rows {
		if personID, external := accountlessExternal(row); external &&
			!schoolmembership.ExternalCaregiverAssignable(false, len(invited[personID]) > 0) {
			continue
		}
		result = append(result, scanTeacher(row))
	}
	return result, nil
}

// accountlessExternal reports the person of an external caregiver without an
// account; the raw guest marker is an active guest profile.
func accountlessExternal(teacher *peopledirectory.Teacher) (int64, bool) {
	if teacher == nil || teacher.Staff == nil || !teacher.Staff.IsGuest {
		return 0, false
	}
	person := teacher.Staff.Person
	if person == nil || person.AccountID != nil {
		return 0, false
	}
	return person.ID, true
}

// invitedExternals reads the open invitations by person, only when an
// external caregiver without an account is on the list.
func (p DeviceScanPeople) invitedExternals(ctx context.Context, rows []*peopledirectory.Teacher) (map[int64][]identityaccess.SchoolInvitation, error) {
	needed := false
	for _, row := range rows {
		if _, external := accountlessExternal(row); external {
			needed = true
			break
		}
	}
	if !needed {
		return nil, nil
	}
	if p.Invitations == nil {
		return nil, errors.New("kiosk teacher list: open invitations are not bound")
	}
	pending, err := p.Invitations.ListPendingSchoolInvitations(ctx)
	if err != nil {
		return nil, err
	}
	return identityaccess.PendingInvitationsByPerson(pending), nil
}

func (p DeviceScanPeople) GetStudentByIDForUpdate(ctx context.Context, id int64) (*devicescan.ScanStudent, error) {
	student, err := p.Users.GetStudentByIDForUpdate(ctx, id)
	return scanStudent(student), err
}

func (p DeviceScanPeople) GetStudentByPersonID(ctx context.Context, personID int64) (*devicescan.ScanStudent, error) {
	student, err := p.Users.StudentRows().FindByPersonID(ctx, personID)
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

func scanPerson(person *peopledirectory.DirectoryPersonSnapshot) *devicescan.ScanPerson {
	if person == nil {
		return nil
	}
	return &devicescan.ScanPerson{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName, TagID: person.TagID}
}

func scanStaff(staff *peopledirectory.Staff) *devicescan.ScanStaff {
	if staff == nil {
		return nil
	}
	return &devicescan.ScanStaff{ID: staff.ID, PersonID: staff.PersonID, Person: scanPerson(staff.Person)}
}

func scanTeacher(teacher *peopledirectory.Teacher) *devicescan.ScanTeacher {
	if teacher == nil {
		return nil
	}
	return &devicescan.ScanTeacher{StaffID: teacher.StaffID, Role: teacher.Role, Specialization: teacher.Specialization, Staff: scanStaff(teacher.Staff)}
}

func scanStudent(student *peopledirectory.StudentState) *devicescan.ScanStudent {
	if student == nil {
		return nil
	}
	return &devicescan.ScanStudent{
		ID: student.ID, PersonID: student.PersonID, GroupID: student.GroupID, SchoolClass: student.SchoolClass,
		Alumnus: student.IsAlumnus(), EnrolledUntil: student.EnrolledUntil,
		Person: scanPerson(student.Person),
	}
}

func (DeviceScanPeople) IsPersonNotFound(err error) bool { return peoplePorts.IsPersonNotFound(err) }
