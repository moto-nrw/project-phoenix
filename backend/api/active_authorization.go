package api

import (
	"context"

	presenceAPI "github.com/moto-nrw/project-phoenix/modules/studentpresence/inbound/presence"
	"github.com/moto-nrw/project-phoenix/services"
)

func authorizeActiveVisit(ctx context.Context, accountID int64, readAll bool, visitID int64, facts presenceAPI.VisitAccessQuery) (bool, error) {
	return services.AuthorizeVisit(ctx, accountID, readAll, visitID, facts)
}

func activeAuthorization() presenceAPI.Authorization {
	return presenceAPI.Authorization{
		Visit: authorizeActiveVisit,
		OperationalOverview: func(ctx context.Context, settings presenceAPI.Settings, staff presenceAPI.StaffAccess, assignmentBound, admin bool) (bool, error) {
			return services.AuthorizeOperationalOverview(ctx, settings, staff, assignmentBound, admin)
		},
	}
}

func activeRequestRuntime() presenceAPI.RequestRuntime {
	return presenceAPI.RequestRuntime{
		WithStaff:    services.WithAttendanceStaff,
		TenantID:     services.AttendanceTenantID,
		MarkRollback: services.MarkAttendanceRollback,
	}
}
