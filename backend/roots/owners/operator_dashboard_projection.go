package owners

import (
	"context"

	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"

	"github.com/moto-nrw/project-phoenix/modules/operatordashboard/contract"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	projection "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/projections/operatordashboard"
)

func newOperatorDashboard(counts func(context.Context, map[int64][]int64) (map[int64]int, error)) organizationtenancy.OperatorDashboard {
	return projection.Map(projection.New(organizationCompose.OperatorDatabase(), counts), contract.Rows[organizationtenancy.DashboardCounts, organizationtenancy.DashboardOrganization, organizationtenancy.DashboardSchool, organizationtenancy.DashboardPWAUsage]{
		Counts: func(v contract.Counts) organizationtenancy.DashboardCounts {
			return organizationtenancy.DashboardCounts(v)
		},
		Organization: func(v contract.OrganizationSummary) organizationtenancy.DashboardOrganization {
			return organizationtenancy.DashboardOrganization(v)
		},
		School: func(v contract.SchoolSummary) organizationtenancy.DashboardSchool {
			return organizationtenancy.DashboardSchool(v)
		},
		PWAUsage: func(v contract.PWAUsageRow) organizationtenancy.DashboardPWAUsage {
			return organizationtenancy.DashboardPWAUsage(v)
		},
	})
}
