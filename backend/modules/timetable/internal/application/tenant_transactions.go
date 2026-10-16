package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// inTenant runs fn in the tenant transaction of tenantID, the way the
// services called the tenant runtime with their database handle before the
// port existed.
func inTenant(ctx context.Context, transactions ports.TenantTransactions, tenantID int64, fn func(context.Context) error) error {
	return transactions.InTenant(ctx, tenantID, fn)
}
