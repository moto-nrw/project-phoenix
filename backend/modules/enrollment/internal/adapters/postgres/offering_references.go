package postgres

import (
	"context"

	"github.com/uptrace/bun"
)

func (r *Store) ReferencedOfferingIDs(ctx context.Context, ids []int64) ([]int64, error) {
	school, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	found := []int64{}
	if len(ids) == 0 {
		return found, nil
	}
	err = db.NewRaw(`SELECT DISTINCT care_offering_id FROM enrollment.request_child_offering_selections
  WHERE tenant_id = ? AND care_offering_id IN (?)`, school, bun.List(ids)).Scan(ctx, &found)
	return found, err
}
