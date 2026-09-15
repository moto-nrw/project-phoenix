package common

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
)

func RequireActiveGroupRead() Middleware   { return RequiresPermission(permissions.GroupsRead) }
func RequireActiveGroupCreate() Middleware { return RequiresPermission(permissions.GroupsCreate) }
func RequireActiveGroupUpdate() Middleware { return RequiresPermission(permissions.GroupsUpdate) }
func RequireActiveGroupDelete() Middleware { return RequiresPermission(permissions.GroupsDelete) }
func RequireActiveGroupAssign() Middleware { return RequiresPermission(permissions.GroupsAssign) }
func RequireVisitUpdate() Middleware       { return RequiresPermission(permissions.VisitsUpdate) }

// VisitReadAccess projects the authenticated caller's legacy broad visit grant.
// A resource wildcard alone is not a broad grant for this relationship policy.
func VisitReadAccess(ctx context.Context) (accountID int64, readAll bool, err error) {
	principal, err := CurrentPrincipal(ctx)
	if err != nil {
		return 0, false, err
	}
	granted := principal.Permissions()
	readAll = slices.Contains(principal.Roles(), "admin") || principal.HasAdminScope() ||
		slices.Contains(granted, permissions.VisitsRead) || slices.Contains(granted, permissions.VisitsManage)
	return principal.AccountID(), readAll, nil
}

// CanUseSchoolWideAttendance checks the request identity only; the attendance
// service still evaluates tenant settings and resource-level access.
func CanUseSchoolWideAttendance(ctx context.Context, staffTenantID, requestTenantID int64) bool {
	principal, err := CurrentPrincipal(ctx)
	ogsScope := principal.Scope() == permissions.ScopeTenant || principal.Scope() == permissions.ScopeOrganization
	return err == nil && ogsScope && principal.AccountID() > 0 && principal.TenantID() > 0 &&
		principal.TenantID() == requestTenantID && staffTenantID == principal.TenantID() &&
		HasPermission(permissions.VisitsUpdate, principal.Permissions())
}
