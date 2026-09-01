package authorize

import (
	"context"
	"fmt"
)

const (
	operationalOverviewScopeKey = "operations.operational_overview_scope"
	OverviewScopeOwn            = "own"
	OverviewScopeAdmins         = "admins"
	OverviewScopeAllStaff       = "all_staff"
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
// OperationalOverviewScope reports the tenant's configured scope for the
// operational overview. An unknown or empty value collapses to
// OverviewScopeOwn — the restrictive default.
//
// A school-portal token always collapses to OverviewScopeOwn regardless of the
// tenant setting: #2380 opens every running module to the OGS staff of a
// school, #2527 deliberately does NOT extend that to Lehrkräfte, whose reach
// stays bound to their own assignments.
func OperationalOverviewScope(ctx context.Context, settings OverviewSettingsResolver, assignmentBound bool) (string, error) {
	if assignmentBound {
		return OverviewScopeOwn, nil
	}
	if settings == nil {
		return OverviewScopeOwn, nil
	}
	scope, err := settings.ResolveString(ctx, operationalOverviewScopeKey)
	if err != nil {
		return OverviewScopeOwn, fmt.Errorf("resolve operational overview scope: %w", err)
	}
	switch scope {
	case OverviewScopeAdmins, OverviewScopeAllStaff:
		return scope, nil
	default:
		return OverviewScopeOwn, nil
	}
}

// HasOperationalOverview decides whether the caller may see every running
// module of the current tenant. Action permissions remain independent.
//
// It never grants a permission: route-level middleware still decides WHICH
// actions the caller may perform, and the tenant transaction still bounds the
// answer to the caller's own school.
//
// The staff lookup is only performed for ordinary callers in all_staff mode.
func HasOperationalOverview(
	ctx context.Context,
	settings OverviewSettingsResolver,
	userCtx StudentAccessUserContext,
	assignmentBound bool,
	admin bool,
) (bool, error) {
	return hasOperationalOverview(ctx, settings, assignmentBound, admin, func() (bool, error) {
		return isVerifiedStaff(ctx, userCtx), nil
	})
}

// HasOperationalOverviewForResolvedStaff applies the same overview policy
// when the caller's staff identity has already been resolved by the service.
func HasOperationalOverviewForResolvedStaff(
	ctx context.Context,
	settings OverviewSettingsResolver,
	assignmentBound bool,
	admin bool,
	hasStaff bool,
) (bool, error) {
	return hasOperationalOverview(ctx, settings, assignmentBound, admin, func() (bool, error) {
		return hasStaff, nil
	})
}

func hasOperationalOverview(
	ctx context.Context,
	settings OverviewSettingsResolver,
	assignmentBound bool,
	admin bool,
	staff func() (bool, error),
) (bool, error) {
	if settings == nil {
		return false, nil
	}
	scope, err := OperationalOverviewScope(ctx, settings, assignmentBound)
	if err != nil {
		return false, err
	}
	if assignmentBound {
		return false, nil
	}
	if admin {
		return true, nil
	}
	if scope != OverviewScopeAllStaff {
		return false, nil
	}
	return staff()
}
