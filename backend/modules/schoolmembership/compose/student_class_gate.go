package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// StudentClassWriteGateQuery supplies School Membership's gate to the
// enrollment identity INSERT. The query is embedded, never executed here.
// This keeps gate-before-row ordering without adding a round trip per child.
func StudentClassWriteGateQuery(ctx context.Context, db *bun.DB) (*bun.SelectQuery, error) {
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return nil, errors.New("school membership: transaction is required")
	}
	return postgres.StudentClassWriteGateQuery(db, tenant.FromContext(ctx))
}
