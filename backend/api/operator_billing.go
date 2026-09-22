package api

import (
	"context"
	"log/slog"

	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	organizationModule "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
)

// newOperatorBilling composes Organisation & Tenancy's billing report
// (#2791) over the two owners whose counts it captures. It needs no graph of
// its own: the operator router and the worker each compose one, and both
// read and write through the ambient administrative transaction.
func newOperatorBilling(logger *slog.Logger) (organizationModule.BillingReport, error) {
	return organizationCompose.NewBilling(organizationCompose.BillingDependencies{
		Counts: billingCounts{
			students:  schoolMembershipCompose.NewActiveStudentCounts().CountActiveStudentsByTenant,
			terminals: devicefleetCompose.NewActiveTerminalCounts().CountActiveTerminalsByTenant,
		},
		Logger: logger.With("service", "billing"),
	})
}

// billingCounts binds the billing report's counts port to its owners:
// School Membership counts the active students, Device Fleet the active
// terminals.
type billingCounts struct {
	students  func(context.Context) (map[int64]int, error)
	terminals func(context.Context) (map[int64]int, error)
}

var _ organizationCompose.BillingCounts = billingCounts{}

func (c billingCounts) CountActiveStudentsByTenant(ctx context.Context) (map[int64]int, error) {
	return c.students(ctx)
}

func (c billingCounts) CountActiveTerminalsByTenant(ctx context.Context) (map[int64]int, error) {
	return c.terminals(ctx)
}
