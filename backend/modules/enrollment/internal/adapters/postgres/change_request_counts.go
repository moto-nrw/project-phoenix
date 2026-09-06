package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func (r *Store) CountChangeRequestsForReview(ctx context.Context, statuses []string) (int, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	count, err := db.NewSelect().TableExpr("enrollment.change_requests").
		Where("tenant_id = ?", tenantID).Where("status IN (?)", bun.List(statuses)).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count enrollment change requests for review: %w", err)
	}
	return count, nil
}
