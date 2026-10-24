package api

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

type attendanceRoutePeopleSource interface {
	FindByAccountID(context.Context, int64) (*peopleValues.DirectoryPersonSnapshot, error)
	StaffRows() peoplePorts.StaffRepository
	TeacherRows() peoplePorts.TeacherRepository
	StudentRows() peoplePorts.StudentRepository
}

type AttendanceRoutePeople struct{ source attendanceRoutePeopleSource }

// NewAttendanceRoutePeople projects the existing tenant-scoped identity lookups.
func NewAttendanceRoutePeople(source attendanceRoutePeopleSource) AttendanceRoutePeople {
	return AttendanceRoutePeople{source: source}
}

func (p AttendanceRoutePeople) FindByAccountID(ctx context.Context, id int64) (int64, bool, error) {
	row, err := p.source.FindByAccountID(ctx, id)
	if row == nil {
		return 0, false, err
	}
	return row.ID, true, err
}

func (p AttendanceRoutePeople) GetStaffByPersonID(ctx context.Context, id int64) (int64, int64, bool, error) {
	row, err := p.source.StaffRows().FindByPersonID(ctx, id)
	if row == nil {
		return 0, 0, false, err
	}
	return row.ID, row.TenantID, true, err
}

func (p AttendanceRoutePeople) GetStudentByPersonID(ctx context.Context, id int64) (int64, *int64, bool, error) {
	row, err := p.source.StudentRows().FindByPersonID(ctx, id)
	if row == nil {
		return 0, nil, false, err
	}
	return row.ID, row.GroupID, true, err
}

func (p AttendanceRoutePeople) GetTeacherByStaffID(ctx context.Context, id int64) (int64, bool, error) {
	row, err := p.source.TeacherRows().FindByStaffID(ctx, id)
	if row == nil {
		return 0, false, err
	}
	return row.ID, true, err
}

func (p AttendanceRoutePeople) GetStudentByID(ctx context.Context, id int64) (int64, *int64, bool, error) {
	row, err := p.source.StudentRows().FindByID(ctx, id)
	if row == nil {
		return 0, nil, false, err
	}
	return row.ID, row.GroupID, true, err
}
