package services

import (
	"context"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// AttendancePrincipal binds attendance to the device authentication projection.
func AttendancePrincipal(ctx context.Context) active.RequestPrincipal {
	return devicescanCompose.AttendancePrincipal(ctx)
}

func WithAttendanceStaff(ctx context.Context, staffID, tenantID int64) context.Context {
	return devicescanCompose.WithAttendanceStaff(ctx, staffID, tenantID)
}

func AttendanceTenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

func MarkAttendanceRollback(ctx context.Context) { tenant.MarkRollback(ctx) }
