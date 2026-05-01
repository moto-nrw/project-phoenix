package enrollment

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

const requestChildOfferingTableExpr = `enrollment.request_child_offerings AS "request_child_offering"`

type RequestChildOfferingRepository struct {
	db *bun.DB
}

func NewRequestChildOfferingRepository(db *bun.DB) enrollment.RequestChildOfferingRepository {
	return &RequestChildOfferingRepository{db: db}
}

// Create inserts a new request_child × care_offering link. PR 7's
// submission service is the primary caller; PR 6 ships only the
// schema + plumbing.
func (r *RequestChildOfferingRepository) Create(ctx context.Context, row *enrollment.RequestChildOffering) error {
	base.EnsureTenantID(ctx, row)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(row).
		ModelTableExpr(requestChildOfferingTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create request child offering: %w", err)
	}
	return nil
}

// ListByRequestChildID returns every offering picked for the given
// child. Admin review (PR 8) and the parent status page (PR 7) call
// this.
func (r *RequestChildOfferingRepository) ListByRequestChildID(ctx context.Context, requestChildID int64) ([]*enrollment.RequestChildOffering, error) {
	var rows []*enrollment.RequestChildOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(requestChildOfferingTableExpr).
		Where(`"request_child_offering".request_child_id = ?`, requestChildID).
		OrderExpr(`"request_child_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request child offerings: %w", err)
	}
	return rows, nil
}

// CountActiveByCareOffering returns the count of children currently
// holding (or competing for) a slot in the given care offering. Joins
// to enrollment.request_children and filters out terminal statuses
// (rejected, withdrawn) so the count reflects what the admin is
// actually managing. Tenant-scoped via RLS on both tables.
func (r *RequestChildOfferingRepository) CountActiveByCareOffering(ctx context.Context, careOfferingID int64) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(requestChildOfferingTableExpr).
		Join(`INNER JOIN enrollment.request_children AS "child" ON "child".id = "request_child_offering".request_child_id`).
		Where(`"request_child_offering".care_offering_id = ?`, careOfferingID).
		Where(`"child".status NOT IN (?)`, bun.In([]string{
			enrollment.ChildStatusRejected,
			enrollment.ChildStatusWithdrawn,
		})).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count active children for care offering %d: %w", careOfferingID, err)
	}
	return count, nil
}
