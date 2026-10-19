package api

import (
	"context"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The adapters below serve the timetable's people ports from the People
// Directory rows (#2729): staff members, persons and the children the
// instance lifecycle checks, in the timetable's own views. Errors pass
// through unchanged, so the timetable keeps classifying a missing row as the
// repository not-found.

// timetableCorrectionPeople serves the attendance correction's person port.
type timetableCorrectionPeople struct{ rows owners.TimetablePersonRows }

func (p timetableCorrectionPeople) FindByAccountID(ctx context.Context, accountID int64) (*timetable.Person, error) {
	return owners.MapRow(owners.TimetablePerson)(p.rows.FindByAccountID(ctx, accountID))
}

// TimetableSupervisorDirectory binds the activity transport's native staff reads.
type TimetableSupervisorDirectory struct {
	rows interface {
		GetStaffWithPersonByIDs(context.Context, []int64) (map[int64]*peopleValues.Staff, error)
		ListStaffWithPerson(context.Context) ([]*peopleValues.Staff, error)
	}
	teachers interface {
		GetTeachersBySpecialization(context.Context, string) ([]*peopleValues.Teacher, error)
	}
}

func NewTimetableSupervisorDirectory(rows interface {
	GetStaffWithPersonByIDs(context.Context, []int64) (map[int64]*peopleValues.Staff, error)
	ListStaffWithPerson(context.Context) ([]*peopleValues.Staff, error)
	GetTeachersBySpecialization(context.Context, string) ([]*peopleValues.Teacher, error)
}) TimetableSupervisorDirectory {
	return TimetableSupervisorDirectory{rows: rows, teachers: rows}
}
func (d TimetableSupervisorDirectory) GetTeachersBySpecialization(ctx context.Context, specialization string) ([]*timetable.SupervisorTeacher, error) {
	rows, err := d.teachers.GetTeachersBySpecialization(ctx, specialization)
	if err != nil {
		return nil, err
	}
	result := make([]*timetable.SupervisorTeacher, len(rows))
	for i, row := range rows {
		if row != nil {
			result[i] = &timetable.SupervisorTeacher{StaffID: row.StaffID}
		}
	}
	return result, nil
}
func (d TimetableSupervisorDirectory) GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.StaffMember, error) {
	return owners.MapRowsByID(owners.TimetableStaffMember)(d.rows.GetStaffWithPersonByIDs(ctx, ids))
}
func (d TimetableSupervisorDirectory) ListStaffWithPerson(ctx context.Context) ([]*timetable.StaffMember, error) {
	rows, err := d.rows.ListStaffWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*timetable.StaffMember, len(rows))
	for i, row := range rows {
		result[i] = owners.TimetableStaffMember(row)
	}
	return result, nil
}
