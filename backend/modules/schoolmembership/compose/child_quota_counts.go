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

// NewChildQuotaCounts composes the cross-school Kontingentzahl of the
// operator school overview (#3568). It reads on the caller's administrative
// transaction and counts on today's Berlin calendar day, the day every
// membership write is checked on.
func NewChildQuotaCounts() schoolmembership.ChildQuotaCounts {
	return childQuotaCounts{today: postgres.BerlinToday}
}

type childQuotaCounts struct {
	today func() string
}

func (c childQuotaCounts) CountChildQuotaByTenant(ctx context.Context) (map[int64]int, error) {
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("school membership: transaction is required")
	}
	if !tenant.IsAdminTx(ctx) {
		return nil, errors.New("school membership: child quota counts need the administrative transaction")
	}
	switch tx := transaction.(type) {
	case bun.Tx:
		return postgres.CountChildQuotaByTenant(ctx, tx, c.today())
	case *bun.Tx:
		if tx != nil {
			return postgres.CountChildQuotaByTenant(ctx, tx, c.today())
		}
	}
	return nil, fmt.Errorf("school membership postgres: unsupported transaction %T", transaction)
}
