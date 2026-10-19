package owners

import (
	"context"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TimetableOperationCaller binds the operational day's caller port to the request's
// session and the shared authorization rules.
type TimetableOperationCaller struct {
	ports.RequestSession
}

func (c TimetableOperationCaller) AssignmentBoundPortal(ctx context.Context) bool {
	return c.Claims(ctx).SchoolScope
}

func (c TimetableOperationCaller) AdministrativeAccess(ctx context.Context) bool {
	claims := c.Claims(ctx)
	return claims.Admin || securityruntime.HasAdminWildcard(c.Permissions(ctx))
}

func (c TimetableOperationCaller) OGSActorToken(ctx context.Context, accountID int64) bool {
	claims := c.Claims(ctx)
	return (claims.Scope == "" || claims.Scope == "tenant" || claims.Scope == "org") &&
		claims.AccountID == accountID && claims.TenantID > 0 && claims.TenantID == tenant.FromContext(ctx) &&
		securityruntime.HasPermission("schedules:read", c.Permissions(ctx))
}

func (TimetableOperationCaller) SchoolWideActionScope(ctx context.Context, settings ports.OperationSettings, scopeKey string) (bool, error) {
	return securityruntime.SchoolWideActionScope(ctx, settings, scopeKey)
}

func (TimetableOperationCaller) OperationalOverview(ctx context.Context, settings ports.OperationSettings, assignmentBound, administrative, hasStaff bool) (bool, error) {
	return securityruntime.HasOperationalOverviewForResolvedStaff(ctx, settings, assignmentBound, administrative, hasStaff)
}

func (TimetableOperationCaller) IsPersonNotFound(err error) bool {
	return peoplePorts.IsPersonNotFound(err)
}
