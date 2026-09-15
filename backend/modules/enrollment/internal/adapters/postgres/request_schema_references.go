package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func (r *Store) CountRequestSchemaReferences(ctx context.Context, ids []int64) (int, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	count, err := db.NewSelect().TableExpr("enrollment.requests").Where("tenant_id = ?", tenantID).Where("schema_id IN (?)", bun.List(ids)).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count request schema references: %w", err)
	}
	return count, nil
}
