package securityruntime

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime/internal/application/authorize"
)

// OverviewSettings resolves tenant policy through the settings registry.
type OverviewSettings interface {
	ResolveString(context.Context, string) (string, error)
}

// OverviewStaffIdentity verifies that the caller has a staff identity.
type OverviewStaffIdentity interface {
	HasCurrentStaff(context.Context) (bool, error)
}

// CanViewOperationalOverview applies school-portal restrictions and tenant
// visibility policy. It does not grant action permissions.
func CanViewOperationalOverview(ctx context.Context, settings OverviewSettings, staff OverviewStaffIdentity, assignmentBound, admin bool) (bool, error) {
	return authorize.HasOperationalOverview(ctx, settings, staff, assignmentBound, admin)
}

// SchoolWideActionScope evaluates the canonical school-wide action policy.
func SchoolWideActionScope(ctx context.Context, settings OverviewSettings, scopeKey string) (bool, error) {
	return authorize.SchoolWideActionScope(ctx, settings, scopeKey)
}

// HasOperationalOverviewForResolvedStaff applies the overview policy to an already verified staff identity.
func HasOperationalOverviewForResolvedStaff(ctx context.Context, settings OverviewSettings, assignmentBound, admin, hasStaff bool) (bool, error) {
	return authorize.HasOperationalOverviewForResolvedStaff(ctx, settings, assignmentBound, admin, hasStaff)
}
