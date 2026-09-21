package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	CurrentStaffID(context.Context) (int64, bool, error)
	HasCurrentStaff(context.Context) (bool, error)
}
type attendanceRouteStaff struct{ source attendanceRouteStaffSource }

// NewAttendanceRouteStaff retains the request-memoized current-staff lookup.
func NewAttendanceRouteStaff(source attendanceRouteStaffSource) attendanceRouteStaff {
	return attendanceRouteStaff{source: source}
}

// GetCurrentStaff reports the caller's staff member in the request's
// tenant, the only tenant the staff read reaches. A caller who is no staff
// member is not found, without an error, so the routes deny access instead
// of reporting a lookup failure.
func (p attendanceRouteStaff) GetCurrentStaff(ctx context.Context) (int64, int64, bool, error) {
	staffID, found, err := p.source.CurrentStaffID(ctx)
	if err != nil || !found {
		return 0, 0, false, err
	}
	return staffID, tenant.FromContext(ctx), true, nil
}
func (p attendanceRouteStaff) HasCurrentStaff(ctx context.Context) (bool, error) {
	return p.source.HasCurrentStaff(ctx)
}
