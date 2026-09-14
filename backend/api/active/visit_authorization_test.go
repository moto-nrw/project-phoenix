package active_test

import (
	"context"

	activeAPI "github.com/moto-nrw/project-phoenix/api/active"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func authorizeVisitForTest(ctx context.Context, accountID int64, readAll bool, visitID int64, facts activeAPI.VisitAccessQuery) (bool, error) {
	return securityruntime.CanViewVisit(ctx, accountID, readAll, visitID, facts)
}

func authorizationForTest() activeAPI.Authorization {
	return activeAPI.Authorization{
		Visit: authorizeVisitForTest,
		OperationalOverview: func(ctx context.Context, settings activeAPI.Settings, staff activeAPI.StaffAccess, assignmentBound, admin bool) (bool, error) {
			return securityruntime.CanViewOperationalOverview(ctx, settings, staff, assignmentBound, admin)
		},
	}
}

func requestRuntimeForTest(withStaff func(context.Context, int64, int64) context.Context) activeAPI.RequestRuntime {
	return activeAPI.RequestRuntime{WithStaff: withStaff, TenantID: tenant.FromContext, MarkRollback: tenant.MarkRollback}
}
