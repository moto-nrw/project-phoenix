package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
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

type attendanceRouteStaffSource interface {
	GetCurrentStaff(context.Context) (*users.Staff, error)
	HasCurrentStaff(context.Context) (bool, error)
}
type attendanceRouteStaff struct{ source attendanceRouteStaffSource }

// NewAttendanceRouteStaff retains the request-memoized current-staff lookup.
func NewAttendanceRouteStaff(source attendanceRouteStaffSource) attendanceRouteStaff {
	return attendanceRouteStaff{source: source}
}
func (p attendanceRouteStaff) GetCurrentStaff(ctx context.Context) (int64, int64, bool, error) {
	row, err := p.source.GetCurrentStaff(ctx)
	if row == nil {
		return 0, 0, false, err
	}
	return row.ID, row.TenantID, true, err
}
func (p attendanceRouteStaff) HasCurrentStaff(ctx context.Context) (bool, error) {
	return p.source.HasCurrentStaff(ctx)
}
