package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	presenceAPI "github.com/moto-nrw/project-phoenix/modules/studentpresence/inbound/presence"
)

func authorizeActiveVisit(ctx context.Context, accountID int64, readAll bool, visitID int64, facts presenceAPI.VisitAccessQuery) (bool, error) {
	return securityruntime.CanViewVisit(ctx, accountID, readAll, visitID, facts)
}

func activeAuthorization() presenceAPI.Authorization {
	return presenceAPI.Authorization{
		Visit:             authorizeActiveVisit,
		StudentDataAccess: activeStudentDataAccess,
		OperationalOverview: func(ctx context.Context, settings presenceAPI.Settings, staff presenceAPI.StaffAccess, assignmentBound, admin bool) (bool, error) {
			return securityruntime.CanViewOperationalOverview(ctx, settings, staff, assignmentBound, admin)
		},
	}
}

func activeRequestRuntime() presenceAPI.RequestRuntime {
	return presenceAPI.RequestRuntime{
		WithStaff:    deviceauth.WithAttendanceStaff,
		TenantID:     devicescanCompose.AttendanceTenantID,
		MarkRollback: devicescanCompose.MarkAttendanceRollback,
	}
}

func activeStudentDataAccess(ctx context.Context, lookup func(context.Context) (bool, error)) (bool, bool) {
	return securityruntime.StudentDataAccess(ctx, sessiontoken.PermissionsFromCtx(ctx), lookup)
}
