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

// StudentClassWriteGate is StudentClassWriteGateQuery bound to db, the gate
// a People Directory composition takes as its class write gate query.
func StudentClassWriteGate(db *bun.DB) func(context.Context) (*bun.SelectQuery, error) {
	return func(ctx context.Context) (*bun.SelectQuery, error) {
		return StudentClassWriteGateQuery(ctx, db)
	}
}
