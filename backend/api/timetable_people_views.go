package api

import (
	"context"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"

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
	rows timetableSupervisorRows
}

// timetableSupervisorRows is the slice of the person directory the
// supervisor reads use: the retained staff and teacher repositories.
type timetableSupervisorRows interface {
	StaffRows() peoplePorts.StaffRepository
	TeacherRows() peoplePorts.TeacherRepository
}

func NewTimetableSupervisorDirectory(rows timetableSupervisorRows) TimetableSupervisorDirectory {
	return TimetableSupervisorDirectory{rows: rows}
}
func (d TimetableSupervisorDirectory) GetTeachersBySpecialization(ctx context.Context, specialization string) ([]*timetable.SupervisorTeacher, error) {
	rows, err := d.rows.TeacherRows().FindBySpecialization(ctx, specialization)
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
	return owners.MapRowsByID(owners.TimetableStaffMember)(d.rows.StaffRows().FindWithPersonByIDs(ctx, ids))
}
func (d TimetableSupervisorDirectory) ListStaffWithPerson(ctx context.Context) ([]*timetable.StaffMember, error) {
	rows, err := d.rows.StaffRows().ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*timetable.StaffMember, len(rows))
	for i, row := range rows {
		result[i] = owners.TimetableStaffMember(row)
	}
	return result, nil
}
