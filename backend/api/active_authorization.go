package api

import (
	"context"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	presenceAPI "github.com/moto-nrw/project-phoenix/modules/studentpresence/inbound/presence"
)

func authorizeActiveVisit(ctx context.Context, accountID int64, readAll bool, visitID int64, facts presenceAPI.VisitAccessQuery) (bool, error) {
	return securityruntime.CanViewVisit(ctx, accountID, readAll, visitID, facts)
}

func activeAuthorization() presenceAPI.Authorization {
	return presenceAPI.Authorization{
		Visit: authorizeActiveVisit,
		OperationalOverview: func(ctx context.Context, settings presenceAPI.Settings, staff presenceAPI.StaffAccess, assignmentBound, admin bool) (bool, error) {
			return securityruntime.CanViewOperationalOverview(ctx, settings, staff, assignmentBound, admin)
		},
	}
}

func activeRequestRuntime() presenceAPI.RequestRuntime {
	return presenceAPI.RequestRuntime{
		WithStaff:    devicescanCompose.WithAttendanceStaff,
		TenantID:     devicescanCompose.AttendanceTenantID,
		MarkRollback: devicescanCompose.MarkAttendanceRollback,
	}
}
