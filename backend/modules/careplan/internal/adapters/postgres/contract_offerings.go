package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ContractOfferingStore reads the offerings for the Betreuungsvertrag
// (#4678) in the caller's tenant transaction.
type ContractOfferingStore struct{}

var _ careplan.ContractOfferings = ContractOfferingStore{}

// NewContractOfferingStore builds the store.
func NewContractOfferingStore() ContractOfferingStore { return ContractOfferingStore{} }

type contractOfferingRow struct {
	ID         int64  `bun:"id"`
	PhaseID    int64  `bun:"phase_id"`
	Name       string `bun:"name"`
	PriceCents *int   `bun:"price_cents"`
	IsActive   bool   `bun:"is_active"`
}

// ContractOfferings lists the offerings of the school.
func (ContractOfferingStore) ContractOfferings(ctx context.Context, ids []int64) ([]careplan.ContractOffering, error) {
	tx, tenantID, err := tenant.CurrentTenantTransaction[bun.Tx](ctx)
	if err != nil {
		return nil, fmt.Errorf("care plan postgres: contract offerings need the school transaction: %w", err)
	}
	if ids == nil {
		ids = []int64{}
	}
	var rows []contractOfferingRow
	if err := tx.NewRaw(`SELECT id, phase_id, name, price_cents, is_active FROM enrollment.care_offerings
		WHERE tenant_id = ? AND (cardinality(?::bigint[]) = 0 OR id = ANY(?::bigint[]))
		ORDER BY sort_order, name, id`, tenantID, pgdialect.Array(ids), pgdialect.Array(ids)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care plan postgres: contract offerings: %w", err)
	}
	result := make([]careplan.ContractOffering, len(rows))
	for i, row := range rows {
		result[i] = careplan.ContractOffering(row)
	}
	return result, nil
}
