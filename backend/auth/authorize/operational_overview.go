package authorize

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// OverviewSettingsResolver is the narrow slice of the settings service the
// operational-overview gate needs. Declared here so this package stays a
// sibling of services/config without a package cycle.
type OverviewSettingsResolver interface {
	ResolveString(ctx context.Context, key string) (string, error)
}

// IsAssignmentBoundPortal reports whether the caller reaches the operational
// surface through a portal whose access follows the Betreuungsplan assignment
// alone — today exactly the school portal ("moto schule", #2527).
//
// A Lehrkraft holds a users.staff row (#2222) and may additionally be an OGS
// admin on the same account. Both facts would otherwise widen her reach the
// moment a school switches the operational overview to "all_staff" or she
// signs in with an admin claim set. The portal she came through is the one
// signal neither can forge, so it — not the role set — decides.
func IsAssignmentBoundPortal(ctx context.Context) bool {
	claims := jwt.ClaimsFromCtx(ctx)
	return claims.IsSchoolScope()
}

// OperationalOverviewScope reports the tenant's configured scope for the
// operational overview. An unknown or empty value collapses to
// OverviewScopeOwn — the restrictive default.
//
// A school-portal token always collapses to OverviewScopeOwn regardless of the
// tenant setting: #2380 opens every running module to the OGS staff of a
// school, #2527 deliberately does NOT extend that to Lehrkräfte, whose reach
// stays bound to their own assignments.
func OperationalOverviewScope(ctx context.Context, settings OverviewSettingsResolver) (string, error) {
	if IsAssignmentBoundPortal(ctx) {
		return configModel.OverviewScopeOwn, nil
	}
	if settings == nil {
		return configModel.OverviewScopeOwn, nil
	}
	scope, err := settings.ResolveString(ctx, configModel.KeyOperationalOverviewScope)
	if err != nil {
		return configModel.OverviewScopeOwn, fmt.Errorf("resolve operational overview scope: %w", err)
	}
	switch scope {
	case configModel.OverviewScopeAdmins, configModel.OverviewScopeAllStaff:
		return scope, nil
	default:
		return configModel.OverviewScopeOwn, nil
	}
}

// HasOperationalOverview is the ONE rule deciding whether the caller may see
// and operate every running module of the current tenant (#2380): lists,
// detail reads, SSE topics and the write actions on those modules all ask
// this same question, so the UI can never show a block whose detail route
// then answers 403.
//
// It never grants a permission: route-level middleware still decides WHICH
// actions the caller may perform, and the tenant transaction still bounds the
// answer to the caller's own school.
//
// The staff lookup is only performed for the all_staff scope, so the common
// own/admins cases stay free of an extra query.
func HasOperationalOverview(
	ctx context.Context,
	settings OverviewSettingsResolver,
	userCtx StudentAccessUserContext,
) (bool, error) {
	scope, err := OperationalOverviewScope(ctx, settings)
	if err != nil {
		return false, err
	}
	switch scope {
	case configModel.OverviewScopeAllStaff:
		if HasEffectiveAdminScope(ctx) {
			return true, nil
		}
		return isVerifiedStaff(ctx, userCtx), nil
	case configModel.OverviewScopeAdmins:
		return HasEffectiveAdminScope(ctx), nil
	default:
		return false, nil
	}
}
