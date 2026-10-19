package owners

import (
	"context"
	"log/slog"
	"time"

	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	organizationModule "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
)

// newOperatorBilling composes Organisation & Tenancy's billing report
// (#2791) over the two owners whose counts it captures. It needs no graph of
// its own: the operator router and the worker each compose one, and both
// read and write through the ambient administrative transaction.
func NewOperatorBilling(logger *slog.Logger) (organizationModule.BillingReport, error) {
	return organizationCompose.NewBilling(organizationCompose.BillingDependencies{
		Counts: BillingCounts{
			Students:  schoolMembershipCompose.NewActiveStudentCounts().CountActiveStudentsByTenant,
			Terminals: devicefleetCompose.NewActiveTerminalCounts().CountActiveTerminalsByTenant,
		},
		Logger: logger.With("service", "billing"),
	})
}

// billingCounts binds the billing report's counts port to its owners:
// School Membership counts the active students, Device Fleet the active
// terminals.
type BillingCounts struct {
	Students  func(context.Context, time.Time) (map[int64]int, error)
	Terminals func(context.Context) (map[int64]int, error)
}

func (c BillingCounts) CountActiveStudentsByTenant(ctx context.Context, capturedAt time.Time) (map[int64]int, error) {
	return c.Students(ctx, capturedAt)
}

func (c BillingCounts) CountActiveTerminalsByTenant(ctx context.Context) (map[int64]int, error) {
	return c.Terminals(ctx)
}
