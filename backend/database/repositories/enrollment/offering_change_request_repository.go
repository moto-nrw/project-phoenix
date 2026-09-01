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
		return nil, &modelBase.DatabaseError{Op: "get pending offering change request", Err: base.TranslateNotFound(err)}
	}
	return row, nil
}

// ListByStudent returns every request of one child, newest decision first.
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
		OrderExpr(`"offering_change_request".reviewed_at DESC NULLS LAST`).
		OrderExpr(`"offering_change_request".id DESC`)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list offering change requests by student", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// ListPendingForTenant returns the tenant's open requests, newest submission
// first, narrowed and paged by filters. The working list orders every queue by
// its submission instant so the aggregated request list can merge them on one
// key (#2432); the effective date is a column of the card, not the sort.
func (r *OfferingChangeRequestRepository) ListPendingForTenant(
	ctx context.Context,
	filters modelBase.RequestQueueFilters,
) ([]*enrollment.OfferingChangeRequest, error) {
	rows := make([]*enrollment.OfferingChangeRequest, 0)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprOfferingChangeRequestsAsReq).
		Where(`"offering_change_request".status = ?`, enrollment.OfferingChangeStatusPending)

	query = base.WithTenantFilter(ctx, query, "offering_change_request")
	query = base.ApplyRequestUrgency(
		query, filters, `"offering_change_request".effective_from <= ?::date`, filters.UrgentDate,
	)
	query = base.ApplyRequestQueueFilters(query, "offering_change_request", "created_at", filters)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list pending offering change requests", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// ListDecidedForTenant returns the tenant's decided requests (approved,
// rejected, withdrawn) newest-decision-first via keyset pagination on
// (updated_at, id). Every Decide stamps updated_at (UpdateDecisionSnapshot runs
// in the same transaction) and decided rows are terminal, so it is the decision
// instant. A zero BeforeInstant returns the first page.
func (r *OfferingChangeRequestRepository) ListDecidedForTenant(
	ctx context.Context,
	filters modelBase.RequestQueueFilters,
) ([]*enrollment.OfferingChangeRequest, error) {
	rows := make([]*enrollment.OfferingChangeRequest, 0)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprOfferingChangeRequestsAsReq).
		Where(`"offering_change_request".status IN (?)`, bun.List([]string{
			enrollment.OfferingChangeStatusApproved,
			enrollment.OfferingChangeStatusRejected,
			enrollment.OfferingChangeStatusWithdrawn,
		}))

	query = base.WithTenantFilter(ctx, query, "offering_change_request")
	query = base.ApplyRequestQueueFilters(query, "offering_change_request", "updated_at", filters)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list decided offering change requests", Err: base.TranslateNotFound(err)}
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
		return nil, &modelBase.DatabaseError{Op: "find offering change request for update", Err: base.TranslateNotFound(err)}
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
		return &modelBase.DatabaseError{Op: "update offering change effective date", Err: base.TranslateNotFound(err)}
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return enrollment.ErrOfferingChangeNotPending
	}
	return nil
}

func (r *OfferingChangeRequestRepository) UpdateApprovedCompleteWithdrawal(
	ctx context.Context,
	id int64,
	complete bool,
) error {
	q := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*enrollment.OfferingChangeRequest)(nil)).
		ModelTableExpr(tableExprOfferingChangeRequestsAsReq).
		Set("approved_complete_withdrawal = ?", complete).
		Set("updated_at = NOW()").
		Where(`"offering_change_request".id = ?`, id).
		Where(`"offering_change_request".status = ?`, enrollment.OfferingChangeStatusPending)
	q = base.WithTenantFilter(ctx, q, "offering_change_request")
	res, err := q.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update approved complete-withdrawal result", Err: base.TranslateNotFound(err)}
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return enrollment.ErrOfferingChangeNotPending
	}
	return nil
}

// Decide transitions a pending row to its final state. The pending predicate
// lives in the WHERE clause so two concurrent reviewers cannot both win.
// UpdatePending rewrites a pending request's selections, effective date and
// note — the guardian edit path (#2267). The pending guard sits in the WHERE
// clause, so an edit racing a staff decision loses.
func (r *OfferingChangeRequestRepository) UpdatePending(
	ctx context.Context,
	id int64,
	payload map[string]any,
	effectiveFrom timezone.Date,
	note *string,
) error {
	q := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*enrollment.OfferingChangeRequest)(nil)).
		ModelTableExpr(tableExprOfferingChangeRequestsAsReq).
		Set("payload = ?", payload).
		Set("effective_from = ?", effectiveFrom).
		Set("parent_note = ?", note).
		Set("updated_at = ?", time.Now()).
		Where(`"offering_change_request".id = ?`, id).
		Where(`"offering_change_request".status = ?`, enrollment.OfferingChangeStatusPending)
	q = base.WithTenantFilter(ctx, q, "offering_change_request")
	res, err := q.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update pending offering change request", Err: base.TranslateNotFound(err)}
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return enrollment.ErrOfferingChangeNotPending
	}
	return nil
}

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
		return &modelBase.DatabaseError{Op: "decide offering change request", Err: base.TranslateNotFound(err)}
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return enrollment.ErrOfferingChangeNotPending
	}
	return nil
}

// UpdateDecisionSnapshot stores the frozen review diff on a decided row (ADR
// 0002). Separate from Decide so the existing race-guarded transition keeps
// its signature; callers write the snapshot in the same transaction.
func (r *OfferingChangeRequestRepository) UpdateDecisionSnapshot(
	ctx context.Context,
	id int64,
	snapshot *enrollment.OfferingChangeDecisionSnapshot,
) error {
	q := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*enrollment.OfferingChangeRequest)(nil)).
		ModelTableExpr(tableExprOfferingChangeRequestsAsReq).
		Set("decision_snapshot = ?", snapshot).
		Set("updated_at = ?", time.Now()).
		Where(`"offering_change_request".id = ?`, id)
	q = base.WithTenantFilter(ctx, q, "offering_change_request")
	if _, err := q.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "update offering change decision snapshot", Err: base.TranslateNotFound(err)}
	}
	return nil
}
