package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

type AttendanceRouteStaffSource interface {
	CurrentStaffID(context.Context) (int64, bool, error)
	HasCurrentStaff(context.Context) (bool, error)
}

// AttendanceRouteStaff answers the attendance routes' current-staff question
// over the request-memoized current-staff lookup.
type AttendanceRouteStaff struct{ source AttendanceRouteStaffSource }

// NewAttendanceRouteStaff retains the request-memoized current-staff lookup.
func NewAttendanceRouteStaff(source AttendanceRouteStaffSource) AttendanceRouteStaff {
	return AttendanceRouteStaff{source: source}
}

// GetCurrentStaff reports the caller's staff member in the request's
// tenant, the only tenant the staff read reaches. A caller who is no staff
// member is not found, without an error, so the routes deny access instead
// of reporting a lookup failure.
func (p AttendanceRouteStaff) GetCurrentStaff(ctx context.Context) (int64, int64, bool, error) {
	staffID, found, err := p.source.CurrentStaffID(ctx)
	if err != nil || !found {
		return 0, 0, false, err
	}
	return staffID, tenant.FromContext(ctx), true, nil
}

// HasCurrentStaff reports whether the caller is a staff member of the
// request's tenant.
func (p AttendanceRouteStaff) HasCurrentStaff(ctx context.Context) (bool, error) {
	return p.source.HasCurrentStaff(ctx)
}
