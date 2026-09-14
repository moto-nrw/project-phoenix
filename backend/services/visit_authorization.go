package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// AuthorizeVisit binds the API's relationship facts to Security Runtime policy.
func AuthorizeVisit(ctx context.Context, accountID int64, readAll bool, visitID int64, facts securityruntime.VisitAccessQuery) (bool, error) {
	return securityruntime.CanViewVisit(ctx, accountID, readAll, visitID, facts)
}

// AuthorizeOperationalOverview binds tenant settings and staff identity to the policy.
func AuthorizeOperationalOverview(ctx context.Context, settings securityruntime.OverviewSettings, staff securityruntime.OverviewStaffIdentity, assignmentBound, admin bool) (bool, error) {
	return securityruntime.CanViewOperationalOverview(ctx, settings, staff, assignmentBound, admin)
}
