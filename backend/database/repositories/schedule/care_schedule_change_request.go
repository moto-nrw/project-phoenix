package schedule

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

const tableExprCareScheduleChangeRequestsAsReq = `schedule.care_schedule_change_requests AS "care_schedule_change_request"`

// ErrCareRequestNotPending is returned when no pending row matched the id
// under the current tenant — it was already decided (lost race) or does not
// exist. Services map this to a 409/404.
var ErrCareRequestNotPending = schedule.ErrCareRequestNotPending

// ErrCareRequestNotFound is returned when no request row exists under the
// current tenant.
var ErrCareRequestNotFound = schedule.ErrCareRequestNotFound

// CareScheduleChangeRequestRepository is the tenant-scoped data-access layer
// for parent care-schedule change requests. It embeds the generic
// base.Repository for Create/FindByID and adds the listing + decision helpers
// the parent and staff review flows need.
type CareScheduleChangeRequestRepository struct {
	*base.Repository[*schedule.CareScheduleChangeRequest]
}

// NewCareScheduleChangeRequestRepository wires a fresh repository.
func NewCareScheduleChangeRequestRepository(db *bun.DB) schedule.CareScheduleChangeRequestRepository {
	repo := base.NewRepository[*schedule.CareScheduleChangeRequest](db, "schedule.care_schedule_change_requests", "CareScheduleChangeRequest")
	repo.TenantScoped = true
	return &CareScheduleChangeRequestRepository{Repository: repo}
}

// GetPendingForStudent returns the student's open request, or nil when none
// exists. The partial unique index guarantees at most one.
func (r *CareScheduleChangeRequestRepository) GetPendingForStudent(ctx context.Context, studentID int64) (*schedule.CareScheduleChangeRequest, error) {
	row := new(schedule.CareScheduleChangeRequest)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(row).
		ModelTableExpr(tableExprCareScheduleChangeRequestsAsReq).
		Where(`"care_schedule_change_request".student_id = ?`, studentID).
		Where(`"care_schedule_change_request".status = ?`, schedule.CareRequestStatusPending)

	query = base.WithTenantFilter(ctx, query, "care_schedule_change_request")

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "get pending care schedule change request", Err: err}
	}
	return row, nil
}

// ListPendingForTenant returns every pending request for the current tenant
// (resolved via RLS / the tenant filter), newest-first.
func (r *CareScheduleChangeRequestRepository) ListPendingForTenant(ctx context.Context) ([]*schedule.CareScheduleChangeRequest, error) {
	var rows []*schedule.CareScheduleChangeRequest
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprCareScheduleChangeRequestsAsReq).
		Where(`"care_schedule_change_request".status = ?`, schedule.CareRequestStatusPending)

	query = base.WithTenantFilter(ctx, query, "care_schedule_change_request")
	query = query.
		OrderExpr(`"care_schedule_change_request".created_at DESC`).
		OrderExpr(`"care_schedule_change_request".id DESC`)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list pending care schedule change requests", Err: err}
	}
	return rows, nil
}

// FindPendingByIDForUpdate locks a request row for the current tenant and
// verifies it is still pending. The lock closes the decision race where one
// reviewer could apply live changes after another reviewer (or the guardian's
// withdrawal) already decided the row.
func (r *CareScheduleChangeRequestRepository) FindPendingByIDForUpdate(ctx context.Context, id int64) (*schedule.CareScheduleChangeRequest, error) {
	row := new(schedule.CareScheduleChangeRequest)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(row).
		ModelTableExpr(tableExprCareScheduleChangeRequestsAsReq).
		Where(`"care_schedule_change_request".id = ?`, id).
		For("UPDATE")

	query = base.WithTenantFilter(ctx, query, "care_schedule_change_request")

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCareRequestNotFound
		}
		return nil, &modelBase.DatabaseError{Op: "find pending care schedule change request for update", Err: err}
	}
	if row.Status != schedule.CareRequestStatusPending {
		return nil, ErrCareRequestNotPending
	}
	return row, nil
}

// FindByIDForUpdate locks a request row by id for the current tenant,
// regardless of status, returning ErrCareRequestNotFound when it is absent.
// Unlike FindPendingByIDForUpdate it does NOT collapse a terminal status into
// ErrCareRequestNotPending, so the guardian withdraw path can verify ownership
// on the locked row first and only then decide whether the (owned) request is
// still pending — a foreign request's id stays reported as not-found.
func (r *CareScheduleChangeRequestRepository) FindByIDForUpdate(ctx context.Context, id int64) (*schedule.CareScheduleChangeRequest, error) {
	row := new(schedule.CareScheduleChangeRequest)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(row).
		ModelTableExpr(tableExprCareScheduleChangeRequestsAsReq).
		Where(`"care_schedule_change_request".id = ?`, id).
		For("UPDATE")

	query = base.WithTenantFilter(ctx, query, "care_schedule_change_request")

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCareRequestNotFound
		}
		return nil, &modelBase.DatabaseError{Op: "find care schedule change request for update", Err: err}
	}
	return row, nil
}

// Decide transitions a pending row to its final state. reviewed_at is stamped
// to now for staff decisions; applied_at only when the weekly plan was
// actually written (approvals). Guardian withdrawals pass reviewedBy = nil
// and get no reviewer stamp.
func (r *CareScheduleChangeRequestRepository) Decide(ctx context.Context, id int64, newStatus string, reason *string, reviewedBy *int64, applied bool) error {
	now := time.Now()
	q := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*schedule.CareScheduleChangeRequest)(nil)).
		ModelTableExpr(tableExprCareScheduleChangeRequestsAsReq).
		Set("status = ?", newStatus).
		Set("decision_reason = ?", reason).
		Set("updated_at = ?", now).
		Where(`"care_schedule_change_request".id = ?`, id).
		Where(`"care_schedule_change_request".status = ?`, schedule.CareRequestStatusPending)

	if reviewedBy != nil && *reviewedBy > 0 {
		q = q.Set("reviewed_by = ?", *reviewedBy).Set("reviewed_at = ?", now)
	}
	if applied {
		q = q.Set("applied_at = ?", now)
	}
	q = base.WithTenantFilter(ctx, q, "care_schedule_change_request")

	res, err := q.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "decide care schedule change request", Err: err}
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		// No pending row with this id under the current tenant — already
		// decided, withdrawn, or not found.
		return ErrCareRequestNotPending
	}
	return nil
}
