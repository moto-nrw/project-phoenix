package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

const tableExprOfferingChangeRequestsAsReq = `enrollment.offering_change_requests AS "offering_change_request"`

// OfferingChangeRequestRepository is the tenant-scoped data-access layer for
// post-enrollment care/AG change requests (#1665). It embeds the generic
// base.Repository for Create/FindByID and adds the listing plus guarded
// decision helpers the parents portal and the staff review queue need.
type OfferingChangeRequestRepository struct {
	*base.Repository[*enrollment.OfferingChangeRequest]
}

// NewOfferingChangeRequestRepository wires a fresh repository.
func NewOfferingChangeRequestRepository(db *bun.DB) enrollment.OfferingChangeRequestRepository {
	repo := base.NewRepository[*enrollment.OfferingChangeRequest](
		db,
		"enrollment.offering_change_requests",
		"OfferingChangeRequest",
	)
	repo.TenantScoped = true
	return &OfferingChangeRequestRepository{Repository: repo}
}

// GetPendingForStudent returns the child's open request, or nil when none
// exists.
func (r *OfferingChangeRequestRepository) GetPendingForStudent(
	ctx context.Context,
	studentID int64,
) (*enrollment.OfferingChangeRequest, error) {
	row := new(enrollment.OfferingChangeRequest)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(row).
		ModelTableExpr(tableExprOfferingChangeRequestsAsReq).
		Where(`"offering_change_request".student_id = ?`, studentID).
		Where(`"offering_change_request".status = ?`, enrollment.OfferingChangeStatusPending)

	query = base.WithTenantFilter(ctx, query, "offering_change_request")

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "get pending offering change request", Err: err}
	}
	return row, nil
}

// ListByStudent returns every request of one child, newest first.
func (r *OfferingChangeRequestRepository) ListByStudent(
	ctx context.Context,
	studentID int64,
) ([]*enrollment.OfferingChangeRequest, error) {
	rows := make([]*enrollment.OfferingChangeRequest, 0)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprOfferingChangeRequestsAsReq).
		Where(`"offering_change_request".student_id = ?`, studentID)

	query = base.WithTenantFilter(ctx, query, "offering_change_request")
	query = query.
		OrderExpr(`"offering_change_request".created_at DESC`).
		OrderExpr(`"offering_change_request".id DESC`)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list offering change requests by student", Err: err}
	}
	return rows, nil
}

// ListPendingForTenant returns the tenant's open requests, soonest effective
// date first — that is the order the office has to work them in.
func (r *OfferingChangeRequestRepository) ListPendingForTenant(
	ctx context.Context,
) ([]*enrollment.OfferingChangeRequest, error) {
	rows := make([]*enrollment.OfferingChangeRequest, 0)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprOfferingChangeRequestsAsReq).
		Where(`"offering_change_request".status = ?`, enrollment.OfferingChangeStatusPending)

	query = base.WithTenantFilter(ctx, query, "offering_change_request")
	query = query.
		OrderExpr(`"offering_change_request".effective_from ASC`).
		OrderExpr(`"offering_change_request".created_at DESC`).
		OrderExpr(`"offering_change_request".id DESC`)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list pending offering change requests", Err: err}
	}
	return rows, nil
}

// FindByIDForUpdate locks one row for the current tenant regardless of status.
func (r *OfferingChangeRequestRepository) FindByIDForUpdate(
	ctx context.Context,
	id int64,
) (*enrollment.OfferingChangeRequest, error) {
	row := new(enrollment.OfferingChangeRequest)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(row).
		ModelTableExpr(tableExprOfferingChangeRequestsAsReq).
		Where(`"offering_change_request".id = ?`, id).
		For("UPDATE")

	query = base.WithTenantFilter(ctx, query, "offering_change_request")

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, enrollment.ErrOfferingChangeNotFound
		}
		return nil, &modelBase.DatabaseError{Op: "find offering change request for update", Err: err}
	}
	return row, nil
}

func (r *OfferingChangeRequestRepository) UpdateEffectiveFrom(
	ctx context.Context,
	id int64,
	effectiveFrom timezone.Date,
) error {
	q := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*enrollment.OfferingChangeRequest)(nil)).
		ModelTableExpr(tableExprOfferingChangeRequestsAsReq).
		Set("effective_from = ?", effectiveFrom).
		Set("updated_at = NOW()").
		Where(`"offering_change_request".id = ?`, id).
		Where(`"offering_change_request".status = ?`, enrollment.OfferingChangeStatusPending)
	q = base.WithTenantFilter(ctx, q, "offering_change_request")
	res, err := q.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update offering change effective date", Err: err}
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return enrollment.ErrOfferingChangeNotPending
	}
	return nil
}

// Decide transitions a pending row to its final state. The pending predicate
// lives in the WHERE clause so two concurrent reviewers cannot both win.
func (r *OfferingChangeRequestRepository) Decide(
	ctx context.Context,
	id int64,
	newStatus string,
	reason *string,
	reviewedBy *int64,
	applied bool,
) error {
	now := time.Now()
	q := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*enrollment.OfferingChangeRequest)(nil)).
		ModelTableExpr(tableExprOfferingChangeRequestsAsReq).
		Set("status = ?", newStatus).
		Set("decision_reason = ?", reason).
		Set("updated_at = ?", now).
		Where(`"offering_change_request".id = ?`, id).
		Where(`"offering_change_request".status = ?`, enrollment.OfferingChangeStatusPending)

	if reviewedBy != nil && *reviewedBy > 0 {
		q = q.Set("reviewed_by = ?", *reviewedBy).Set("reviewed_at = ?", now)
	}
	if applied {
		q = q.Set("applied_at = ?", now)
	}
	q = base.WithTenantFilter(ctx, q, "offering_change_request")

	res, err := q.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "decide offering change request", Err: err}
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return enrollment.ErrOfferingChangeNotPending
	}
	return nil
}
