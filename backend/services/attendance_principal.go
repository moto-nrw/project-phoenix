package services

import (
	"context"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/services/active"
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

// WithAttendanceDevice binds a verified kiosk device to the attendance context.
func WithAttendanceDevice(ctx context.Context, deviceID, tenantID int64) context.Context {
	return devicescanCompose.WithAttendanceDevice(ctx, deviceID, tenantID)
}

// WithIoTAttendanceRequest marks the context as an IoT device request.
func WithIoTAttendanceRequest(ctx context.Context) context.Context {
	return devicescanCompose.WithIoTAttendanceRequest(ctx)
}
