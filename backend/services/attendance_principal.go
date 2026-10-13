package services

import (
	"context"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
)

// AttendancePrincipal binds attendance to the device authentication projection.
func AttendancePrincipal(ctx context.Context) presenceservice.RequestPrincipal {
	return devicescanCompose.AttendancePrincipal(ctx)
}

func WithAttendanceStaff(ctx context.Context, staffID, tenantID int64) context.Context {
	return devicescanCompose.WithAttendanceStaff(ctx, staffID, tenantID)
}
