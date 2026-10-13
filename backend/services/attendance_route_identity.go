package services

import (
	"context"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
)

type attendanceRoutePeopleSource interface {
	FindByAccountID(context.Context, int64) (*users.Person, error)
	GetStaffByPersonID(context.Context, int64) (*users.Staff, error)
	GetStudentByPersonID(context.Context, int64) (*users.Student, error)
	GetTeacherByStaffID(context.Context, int64) (*users.Teacher, error)
	GetStudentByID(context.Context, int64) (*users.Student, error)
}

type attendanceRoutePeople struct{ source attendanceRoutePeopleSource }

// NewAttendanceRoutePeople projects the existing tenant-scoped identity lookups.
func NewAttendanceRoutePeople(source attendanceRoutePeopleSource) attendanceRoutePeople {
	return attendanceRoutePeople{source: source}
}

func (p attendanceRoutePeople) FindByAccountID(ctx context.Context, id int64) (int64, bool, error) {
	row, err := p.source.FindByAccountID(ctx, id)
	if row == nil {
		return 0, false, err
	}
	return row.ID, true, err
}

func (p attendanceRoutePeople) GetStaffByPersonID(ctx context.Context, id int64) (int64, int64, bool, error) {
	row, err := p.source.GetStaffByPersonID(ctx, id)
	if row == nil {
		return 0, 0, false, err
	}
	return row.ID, row.TenantID, true, err
}

func (p attendanceRoutePeople) GetStudentByPersonID(ctx context.Context, id int64) (int64, *int64, bool, error) {
	row, err := p.source.GetStudentByPersonID(ctx, id)
	if row == nil {
		return 0, nil, false, err
	}
	return row.ID, row.GroupID, true, err
}

func (p attendanceRoutePeople) GetTeacherByStaffID(ctx context.Context, id int64) (int64, bool, error) {
	row, err := p.source.GetTeacherByStaffID(ctx, id)
	if row == nil {
		return 0, false, err
	}
	return row.ID, true, err
}

func (p attendanceRoutePeople) GetStudentByID(ctx context.Context, id int64) (int64, *int64, bool, error) {
	row, err := p.source.GetStudentByID(ctx, id)
	if row == nil {
		return 0, nil, false, err
	}
	return row.ID, row.GroupID, true, err
}
