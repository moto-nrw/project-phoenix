package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewActiveStudentCounts composes the cross-school active student count of
// the operator billing report (#2791). It reads on the caller's
// administrative transaction and needs nothing else of the owner.
func NewActiveStudentCounts() schoolmembership.ActiveStudentCounts {
	return activeStudentCounts{}
}

type activeStudentCounts struct{}

func (activeStudentCounts) CountActiveStudentsByTenant(ctx context.Context) (map[int64]int, error) {
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("school membership: transaction is required")
	}
	if !tenant.IsAdminTx(ctx) {
		return nil, errors.New("school membership: active student counts need the administrative transaction")
	}
	switch tx := transaction.(type) {
	case bun.Tx:
		return postgres.CountActiveStudentsByTenant(ctx, tx)
	case *bun.Tx:
		if tx != nil {
			return postgres.CountActiveStudentsByTenant(ctx, tx)
		}
	}
	return nil, fmt.Errorf("school membership postgres: unsupported transaction %T", transaction)
}
