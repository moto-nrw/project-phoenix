package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/userscontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// operationCaller binds the operational day's caller port to the request's
// session token and the shared authorization rules.
type operationCaller struct{}

func (operationCaller) AssignmentBoundPortal(ctx context.Context) bool {
	claims := sessiontoken.ClaimsFromCtx(ctx)
	return claims.IsSchoolScope()
}

func (operationCaller) AdministrativeAccess(ctx context.Context) bool {
	claims := sessiontoken.ClaimsFromCtx(ctx)
	return claims.IsAdmin || authorize.HasAdminWildcard(sessiontoken.PermissionsFromCtx(ctx))
}

func (operationCaller) OGSActorToken(ctx context.Context, accountID int64) bool {
	claims := sessiontoken.ClaimsFromCtx(ctx)
	return (claims.Scope == "" || claims.Scope == "tenant" || claims.Scope == "org") &&
		int64(claims.ID) == accountID && claims.TenantID > 0 && claims.TenantID == tenant.FromContext(ctx) &&
		authorize.HasPermission("schedules:read", sessiontoken.PermissionsFromCtx(ctx))
}

func (operationCaller) SchoolWideActionScope(ctx context.Context, settings ports.OperationSettings, scopeKey string) (bool, error) {
	return authorize.SchoolWideActionScope(ctx, settings, scopeKey)
}

func (operationCaller) OperationalOverview(ctx context.Context, settings ports.OperationSettings, assignmentBound, administrative, hasStaff bool) (bool, error) {
	return authorize.HasOperationalOverviewForResolvedStaff(ctx, settings, assignmentBound, administrative, hasStaff)
}

func (operationCaller) IsPersonNotFound(err error) bool {
	return errors.Is(err, userscontract.ErrPersonNotFound)
}
