package securityruntime

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
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
