package api

import (
	"context"

	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TimetableHTTPTransactions preserves the transport's established transaction boundary.
func TimetableHTTPTransactions(ctx context.Context, id int64, callback func(context.Context) error) error {
	return tenant.WithTenantTx(ctx, struct{}{}, id, transactionruntime.TenantCallback(callback))
}
