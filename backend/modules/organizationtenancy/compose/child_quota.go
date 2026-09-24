package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewChildQuotaLimits composes the tenant-safe read of the Kinderkontingent
// (#3567) that School Membership checks its counting writes against. It reads
// on the caller's tenant transaction and answers only for the tenant in
// context; it needs nothing else of the owner.
func NewChildQuotaLimits() organizationtenancy.ChildQuotaLimits {
	return childQuotaLimits{}
}

type childQuotaLimits struct{}

func (childQuotaLimits) ChildQuotaLimit(ctx context.Context) (int, bool, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, false, fmt.Errorf("organization tenancy: child quota: %w", err)
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return 0, false, errors.New("organization tenancy: child quota: transaction is required")
	}
	var db bun.IDB
	switch tx := transaction.(type) {
	case bun.Tx:
		db = tx
	case *bun.Tx:
		if tx != nil {
			db = tx
		}
	}
	if db == nil {
		return 0, false, fmt.Errorf("organization tenancy: child quota: unsupported transaction %T", transaction)
	}
	bundles, bundleSize, err := postgres.ReadChildQuota(ctx, db, tenantID.Int64())
	if err != nil {
		return 0, false, mapError(err)
	}
	if bundles == nil {
		return 0, false, nil
	}
	return organizationtenancy.ChildQuota{Bundles: *bundles, BundleSize: bundleSize}.Limit(), true, nil
}
