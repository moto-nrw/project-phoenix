package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const tableExprExcusedAbsenceRequestsAsReq = `active.excused_absence_requests AS "excused_absence_request"`

// excusedRequestLockClass salts the per-student advisory lock used by
// LockStudentRequests so it shares no key space with other advisory locks. It is
// the first arg of the two-int pg_advisory_xact_lock form ("exab" in ASCII); the
// second arg is the student id.
const excusedRequestLockClass int32 = 0x65786162

// ErrExcusedRequestNotPending is returned when no pending row matched the id
// under the current tenant — it was already decided (lost race) or does not
// exist. Services map this to a 409/404.
var ErrExcusedRequestNotPending = activeModels.ErrExcusedRequestNotPending

// ErrExcusedRequestNotFound is returned when no request row exists under the
// current tenant.
var ErrExcusedRequestNotFound = activeModels.ErrExcusedRequestNotFound

// ExcusedAbsenceRequestRepository is the tenant-scoped data-access layer for
// parent excused-absence requests. It embeds the generic base.Repository for
// Create/FindByID and adds the listing + decision helpers the parent and staff
// review flows need.
type ExcusedAbsenceRequestRepository struct {
	*base.Repository[*activeModels.ExcusedAbsenceRequest]
}

// NewExcusedAbsenceRequestRepository wires a fresh repository.
func NewExcusedAbsenceRequestRepository(db *bun.DB) activeModels.ExcusedAbsenceRequestRepository {
	repo := base.NewRepository[*activeModels.ExcusedAbsenceRequest](db, "active.excused_absence_requests", "ExcusedAbsenceRequest")
	repo.TenantScoped = true
	return &ExcusedAbsenceRequestRepository{Repository: repo}
}

// LockStudentRequests acquires a per-student pg_advisory_xact_lock so concurrent
// CreateRequest calls for the same child serialize (the service reads pending
// rows then inserts — a race without this lock could produce duplicate or
// contradictory pending requests). Held until the ambient transaction ends.
func (r *ExcusedAbsenceRequestRepository) LockStudentRequests(ctx context.Context, studentID int64) error {
	if studentID <= 0 || studentID > math.MaxInt32 {
		return fmt.Errorf("LockStudentRequests: student_id %d out of advisory-lock range", studentID)
	}
	if _, err := base.GetDB(ctx, r.DB).
		NewRaw("SELECT pg_advisory_xact_lock(?, ?)", excusedRequestLockClass, int32(studentID)).
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "lock student excused requests", Err: err}
	}
	return nil
}

// ListPendingForStudent returns the student's open requests, newest-first.
func (r *ExcusedAbsenceRequestRepository) ListPendingForStudent(ctx context.Context, studentID int64) ([]*activeModels.ExcusedAbsenceRequest, error) {
	var rows []*activeModels.ExcusedAbsenceRequest
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprExcusedAbsenceRequestsAsReq).
		Where(`"excused_absence_request".student_id = ?`, studentID).
		Where(`"excused_absence_request".status = ?`, activeModels.ExcusedRequestStatusPending)

	if where, val, ok := base.TenantWhere(ctx, "excused_absence_request"); ok {
		query = query.Where(where, val)
	}
	query = query.
		OrderExpr(`"excused_absence_request".created_at DESC`).
		OrderExpr(`"excused_absence_request".id DESC`)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list pending excused absence requests for student", Err: err}
	}
	return rows, nil
}

// ListRecentForStudent returns the student's requests decided (or, for still
// -pending rows, last touched) on or after `since`, newest-first, regardless of
// status.
//
// The window filters on the DECISION time, not created_at: a request can sit
// pending for longer than the recent window and only then be rejected or
// withdrawn. Filtering on created_at would drop it from the parent's outcome
// view the instant it leaves the pending list, so a parent would never learn a
// long-pending request was declined. reviewed_at is stamped for staff decisions
// (approve/reject); withdrawals carry no reviewer, so COALESCE falls through to
// updated_at (stamped on every Decide) to cover them. Pending rows keep
// updated_at ≈ created_at and, when older than the window, are still surfaced by
// ListPendingForStudent.
func (r *ExcusedAbsenceRequestRepository) ListRecentForStudent(ctx context.Context, studentID int64, since time.Time) ([]*activeModels.ExcusedAbsenceRequest, error) {
	var rows []*activeModels.ExcusedAbsenceRequest
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprExcusedAbsenceRequestsAsReq).
		Where(`"excused_absence_request".student_id = ?`, studentID).
		Where(`COALESCE("excused_absence_request".reviewed_at, "excused_absence_request".updated_at) >= ?`, since)

	if where, val, ok := base.TenantWhere(ctx, "excused_absence_request"); ok {
		query = query.Where(where, val)
	}
	query = query.
		OrderExpr(`"excused_absence_request".created_at DESC`).
		OrderExpr(`"excused_absence_request".id DESC`)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list recent excused absence requests for student", Err: err}
	}
	return rows, nil
}

// ListPendingForTenant returns the pending requests for the current tenant
// (resolved via RLS / the tenant filter), newest submission first, narrowed and
// paged by filters.
func (r *ExcusedAbsenceRequestRepository) ListPendingForTenant(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*activeModels.ExcusedAbsenceRequest, error) {
	var rows []*activeModels.ExcusedAbsenceRequest
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprExcusedAbsenceRequestsAsReq).
		Where(`"excused_absence_request".status = ?`, activeModels.ExcusedRequestStatusPending)

	if where, val, ok := base.TenantWhere(ctx, "excused_absence_request"); ok {
		query = query.Where(where, val)
	}
	query = base.ApplyRequestQueueFilters(query, "excused_absence_request", "created_at", filters)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list pending excused absence requests", Err: err}
	}
	return rows, nil
}

// ListDecidedForTenant returns the tenant's decided requests (approved,
// rejected, withdrawn) newest-decision-first via keyset pagination on
// (updated_at, id). Every Decide stamps updated_at and decided rows are
// terminal, so it is the decision instant (withdrawn rows carry no
// reviewed_at). A zero BeforeInstant returns the first page.
func (r *ExcusedAbsenceRequestRepository) ListDecidedForTenant(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*activeModels.ExcusedAbsenceRequest, error) {
	var rows []*activeModels.ExcusedAbsenceRequest
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprExcusedAbsenceRequestsAsReq).
		Where(`"excused_absence_request".status IN (?)`, bun.List([]string{
			activeModels.ExcusedRequestStatusApproved,
			activeModels.ExcusedRequestStatusRejected,
			activeModels.ExcusedRequestStatusWithdrawn,
		}))

	if where, val, ok := base.TenantWhere(ctx, "excused_absence_request"); ok {
		query = query.Where(where, val)
	}
	query = base.ApplyRequestQueueFilters(query, "excused_absence_request", "updated_at", filters)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list decided excused absence requests", Err: err}
	}
	return rows, nil
}

// FindPendingByIDForUpdate locks a request row for the current tenant and
// verifies it is still pending. The lock closes the decision race where two
// reviewers (or a decide racing the guardian's withdrawal) could both act.
func (r *ExcusedAbsenceRequestRepository) FindPendingByIDForUpdate(ctx context.Context, id int64) (*activeModels.ExcusedAbsenceRequest, error) {
	row := new(activeModels.ExcusedAbsenceRequest)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(row).
		ModelTableExpr(tableExprExcusedAbsenceRequestsAsReq).
		Where(`"excused_absence_request".id = ?`, id).
		For("UPDATE")

	if where, val, ok := base.TenantWhere(ctx, "excused_absence_request"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrExcusedRequestNotFound
		}
		return nil, &modelBase.DatabaseError{Op: "find pending excused absence request for update", Err: err}
	}
	if row.Status != activeModels.ExcusedRequestStatusPending {
		return nil, ErrExcusedRequestNotPending
	}
	return row, nil
}

// FindByIDForUpdate locks a request row by id for the current tenant regardless
// of status, returning ErrExcusedRequestNotFound when it is absent. Unlike
// FindPendingByIDForUpdate it does NOT collapse a terminal status into
// ErrExcusedRequestNotPending, so the guardian withdraw path can verify
// ownership on the locked row first — a foreign request's id stays reported as
// not-found.
func (r *ExcusedAbsenceRequestRepository) FindByIDForUpdate(ctx context.Context, id int64) (*activeModels.ExcusedAbsenceRequest, error) {
	row := new(activeModels.ExcusedAbsenceRequest)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(row).
		ModelTableExpr(tableExprExcusedAbsenceRequestsAsReq).
		Where(`"excused_absence_request".id = ?`, id).
		For("UPDATE")

	if where, val, ok := base.TenantWhere(ctx, "excused_absence_request"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrExcusedRequestNotFound
		}
		return nil, &modelBase.DatabaseError{Op: "find excused absence request for update", Err: err}
	}
	return row, nil
}

// Decide transitions a pending row to its final state. reviewed_at is stamped
// for staff decisions; applied_at only when the excused status days were
// actually written (approvals). Guardian withdrawals pass reviewedBy = nil.
func (r *ExcusedAbsenceRequestRepository) Decide(ctx context.Context, id int64, newStatus string, reason *string, reviewedBy *int64, applied bool) error {
	now := time.Now()
	q := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*activeModels.ExcusedAbsenceRequest)(nil)).
		ModelTableExpr(tableExprExcusedAbsenceRequestsAsReq).
		Set("status = ?", newStatus).
		Set("decision_reason = ?", reason).
		Set("updated_at = ?", now).
		Where(`"excused_absence_request".id = ?`, id).
		Where(`"excused_absence_request".status = ?`, activeModels.ExcusedRequestStatusPending)

	if reviewedBy != nil && *reviewedBy > 0 {
		q = q.Set("reviewed_by = ?", *reviewedBy).Set("reviewed_at = ?", now)
	}
	if applied {
		q = q.Set("applied_at = ?", now)
	}
	if where, val, ok := base.TenantWhere(ctx, "excused_absence_request"); ok {
		q = q.Where(where, val)
	}

	res, err := q.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "decide excused absence request", Err: err}
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrExcusedRequestNotPending
	}
	return nil
}
