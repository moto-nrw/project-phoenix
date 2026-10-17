package presence_test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	presenceAPI "github.com/moto-nrw/project-phoenix/modules/studentpresence/inbound/presence"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func authorizeVisitForTest(ctx context.Context, accountID int64, readAll bool, visitID int64, facts presenceAPI.VisitAccessQuery) (bool, error) {
	return securityruntime.CanViewVisit(ctx, accountID, readAll, visitID, facts)
}

func authorizationForTest() presenceAPI.Authorization {
	return presenceAPI.Authorization{
		Visit: authorizeVisitForTest,
		StudentDataAccess: func(ctx context.Context, lookup func(context.Context) (bool, error)) (bool, bool) {
			return securityruntime.StudentDataAccess(ctx, testpkg.PermissionsFromContext(ctx), lookup)
		},
		OperationalOverview: func(ctx context.Context, settings presenceAPI.Settings, staff presenceAPI.StaffAccess, assignmentBound, admin bool) (bool, error) {
			return securityruntime.CanViewOperationalOverview(ctx, settings, staff, assignmentBound, admin)
		},
	}
}

func requestRuntimeForTest(withStaff func(context.Context, int64, int64) context.Context) presenceAPI.RequestRuntime {
	return presenceAPI.RequestRuntime{WithStaff: withStaff, TenantID: tenant.FromContext, MarkRollback: tenant.MarkRollback}
}
