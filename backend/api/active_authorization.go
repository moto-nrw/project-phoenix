package api

import (
	"context"

	activeAPI "github.com/moto-nrw/project-phoenix/api/active"
	"github.com/moto-nrw/project-phoenix/services"
)

func authorizeActiveVisit(ctx context.Context, accountID int64, readAll bool, visitID int64, facts activeAPI.VisitAccessQuery) (bool, error) {
	return services.AuthorizeVisit(ctx, accountID, readAll, visitID, facts)
}

func activeAuthorization() activeAPI.Authorization {
	return activeAPI.Authorization{
		Visit: authorizeActiveVisit,
		OperationalOverview: func(ctx context.Context, settings activeAPI.Settings, staff activeAPI.StaffAccess, assignmentBound, admin bool) (bool, error) {
			return services.AuthorizeOperationalOverview(ctx, settings, staff, assignmentBound, admin)
		},
	}
}

func activeRequestRuntime() activeAPI.RequestRuntime {
	return activeAPI.RequestRuntime{
		WithStaff:    services.WithAttendanceStaff,
		TenantID:     services.AttendanceTenantID,
		MarkRollback: services.MarkAttendanceRollback,
	}
}
