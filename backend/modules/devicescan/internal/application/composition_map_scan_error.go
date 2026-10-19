package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/ports"
)

type mappedScanError struct{ err, sentinel error }

func (e mappedScanError) Error() string        { return e.err.Error() }
func (e mappedScanError) Unwrap() error        { return e.err }
func (e mappedScanError) Is(target error) bool { return target == e.sentinel }
func MapScanError(err, sentinel error) error   { return mappedScanError{err: err, sentinel: sentinel} }
func FindPersonByTagBinding(ctx context.Context, source devicescan.PersonLookup, tag string) (*ports.Person, error) {
	person, err := source.FindByTagID(ctx, tag)
	if err != nil {
		if source.IsPersonNotFound(err) {
			return nil, MapScanError(err, ports.ErrPersonNotFound)
		}
		return nil, err
	}
	if person == nil {
		return nil, ports.ErrPersonNotFound
	}
	return &ports.Person{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName, HasTag: person.TagID != nil, TagID: person.TagID}, nil
}
func FindStudentByPersonBinding(ctx context.Context, source devicescan.StudentLookup, noRows error, personID int64) (*ports.Student, error) {
	student, err := source.GetStudentByPersonID(ctx, personID)
	if err != nil {
		if errors.Is(err, noRows) {
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
func FindStaffByPersonBinding(ctx context.Context, source devicescan.StaffLookup, noRows error, personID int64) (*ports.StaffMember, error) {
	staff, err := source.GetStaffByPersonID(ctx, personID)
	if err != nil {
		if errors.Is(err, noRows) {
			return nil, nil
		}
		return nil, err
	}
	if staff == nil {
		return nil, nil
	}
	return &ports.StaffMember{ID: staff.ID, PersonID: staff.PersonID}, nil
}
func TeachersBinding(ctx context.Context, source devicescan.StaffLookup) ([]DirectoryTeacher, error) {
	teachers, err := source.ListTeachersWithStaffAndPerson(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]DirectoryTeacher, 0, len(teachers))
	for _, teacher := range teachers {
		row := DirectoryTeacher{}
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
func FindStaffBinding(ctx context.Context, source devicescan.StaffLookup, staffID int64) (*ports.StaffMember, error) {
	staff, err := source.GetStaffByID(ctx, staffID)
	if err != nil || staff == nil {
		return nil, err
	}
	return &ports.StaffMember{ID: staff.ID, PersonID: staff.PersonID}, nil
}
func FindPersonBinding(ctx context.Context, source devicescan.PersonLookup, personID int64) (*ports.Person, error) {
	person, err := source.FindPersonByID(ctx, personID)
	if err != nil || person == nil {
		return nil, err
	}
	return &ports.Person{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName, HasTag: person.TagID != nil, TagID: person.TagID}, nil
}
func TeacherRoleBinding(ctx context.Context, source devicescan.StaffLookup, staffID int64) (string, string, bool, error) {
	teacher, err := source.GetTeacherByStaffID(ctx, staffID)
	if err != nil || teacher == nil {
		return "", "", false, err
	}
	return teacher.Role, teacher.Specialization, true, nil
}
func LockFeedbackStudentBinding(ctx context.Context, source devicescan.StudentLookup, noRows error, studentID int64) (devicescan.FeedbackStudent, error) {
	student, err := source.GetStudentByIDForUpdate(ctx, studentID)
	if errors.Is(err, noRows) || (err == nil && student == nil) {
		return devicescan.FeedbackStudent{}, devicescan.ErrFeedbackStudentNotFound
	}
	if err != nil {
		return devicescan.FeedbackStudent{}, err
	}
	return devicescan.FeedbackStudent{ID: student.ID, Alumnus: student.Alumnus}, nil
}
func StaffNamesBinding(ctx context.Context, source devicescan.StaffLookup, ids []int64) (map[int64]ports.Person, error) {
	staff, err := source.GetStaffWithPersonByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]ports.Person, len(staff))
	for id, row := range staff {
		if row != nil && row.Person != nil {
			result[id] = ports.Person{ID: row.Person.ID, FirstName: row.Person.FirstName, LastName: row.Person.LastName}
		}
	}
	return result, nil
}
