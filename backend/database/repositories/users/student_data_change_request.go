package users

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const tableExprStudentDataChangeRequestsAsReq = `users.student_data_change_requests AS "student_data_change_request"`

// ErrChangeRequestNotPending is returned by Decide when no pending row matched
// the id under the current tenant — it was already decided (lost race) or does
// not exist. Services map this to a 409/404.
var ErrChangeRequestNotPending = users.ErrChangeRequestNotPending

// ErrChangeRequestNotFound is returned when no change request row exists under
// the current tenant.
var ErrChangeRequestNotFound = users.ErrChangeRequestNotFound

// ErrChangeRequestNotDecided is the repository alias of the model sentinel.
var ErrChangeRequestNotDecided = users.ErrChangeRequestNotDecided

// StudentDataChangeRequestRepository is the tenant-scoped data-access layer for
// parent Stammdaten changes. It embeds the generic base.Repository for
// Create/FindByID and adds the listing + decision helpers the parent and staff
// flows need.
type StudentDataChangeRequestRepository struct {
	*base.Repository[*users.StudentDataChangeRequest]
}

// NewStudentDataChangeRequestRepository wires a fresh repository.
func NewStudentDataChangeRequestRepository(db *bun.DB) users.StudentDataChangeRequestRepository {
	repo := base.NewRepository[*users.StudentDataChangeRequest](db, "users.student_data_change_requests", "StudentDataChangeRequest")
	repo.TenantScoped = true
	return &StudentDataChangeRequestRepository{Repository: repo}
}

// ListByStudent returns the student's change rows newest-first, optionally
// filtered to a status set. limit <= 0 returns every matching row.
func (r *StudentDataChangeRequestRepository) ListByStudent(ctx context.Context, studentID int64, statuses []string, limit int) ([]*users.StudentDataChangeRequest, error) {
	var rows []*users.StudentDataChangeRequest
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprStudentDataChangeRequestsAsReq).
		Where(`"student_data_change_request".student_id = ?`, studentID)

	if len(statuses) > 0 {
		query = query.Where(`"student_data_change_request".status IN (?)`, bun.List(statuses))
	}
	query = base.WithTenantFilter(ctx, query, "student_data_change_request")
	query = query.
		OrderExpr(`"student_data_change_request".created_at DESC`).
		OrderExpr(`"student_data_change_request".id DESC`)
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list student data change requests", Err: err}
	}
	return rows, nil
}

// ListParentVisibleByStudent returns only child-level Track B rows that are safe
// to expose to any linked parent of the child.
func (r *StudentDataChangeRequestRepository) ListParentVisibleByStudent(ctx context.Context, studentID int64, limit int) ([]*users.StudentDataChangeRequest, error) {
	var rows []*users.StudentDataChangeRequest
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprStudentDataChangeRequestsAsReq).
		Where(`"student_data_change_request".student_id = ?`, studentID).
		Where(`"student_data_change_request".target IN (?)`, bun.List([]string{
			users.DataChangeTargetPerson,
			users.DataChangeTargetDeparture,
		}))

	query = base.WithTenantFilter(ctx, query, "student_data_change_request")
	query = query.
		OrderExpr(`"student_data_change_request".created_at DESC`).
		OrderExpr(`"student_data_change_request".id DESC`)
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list parent-visible student data change requests", Err: err}
	}
	return rows, nil
}

// ListPendingForTenant returns the pending Track B rows for the current tenant
// (resolved via RLS / the tenant filter), newest submission first, narrowed and
// paged by filters.
func (r *StudentDataChangeRequestRepository) ListPendingForTenant(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*users.StudentDataChangeRequest, error) {
	var rows []*users.StudentDataChangeRequest
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprStudentDataChangeRequestsAsReq).
		Where(`"student_data_change_request".status = ?`, users.DataChangeStatusPending)

	query = base.WithTenantFilter(ctx, query, "student_data_change_request")
	query = base.ApplyRequestUrgency(query, filters, "FALSE")
	query = base.ApplyRequestQueueFilters(query, "student_data_change_request", "created_at", filters)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list pending student data change requests", Err: err}
	}
	return rows, nil
}

// ListDecidedForTenant returns the tenant's decided Track B rows (auto-applied,
// approved, rejected) newest-decision-first via keyset pagination on
// (updated_at, id). updated_at is stamped by every decide/auto-apply path and
// decided rows are terminal, so it is the decision instant. A zero
// BeforeInstant returns the first page.
func (r *StudentDataChangeRequestRepository) ListDecidedForTenant(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*users.StudentDataChangeRequest, error) {
	var rows []*users.StudentDataChangeRequest
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprStudentDataChangeRequestsAsReq).
		Where(`"student_data_change_request".status IN (?)`, bun.List([]string{
			users.DataChangeStatusAutoApplied,
			users.DataChangeStatusApproved,
			users.DataChangeStatusRejected,
		}))

	query = base.WithTenantFilter(ctx, query, "student_data_change_request")
	query = base.ApplyRequestQueueFilters(query, "student_data_change_request", "updated_at", filters)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list decided student data change requests", Err: err}
	}
	return rows, nil
}

// HasPendingForField reports whether an undecided pending row already exists for
// the same student/target/field.
func (r *StudentDataChangeRequestRepository) HasPendingForField(ctx context.Context, studentID int64, target, fieldKey string) (bool, error) {
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model((*users.StudentDataChangeRequest)(nil)).
		ModelTableExpr(tableExprStudentDataChangeRequestsAsReq).
		Where(`"student_data_change_request".student_id = ?`, studentID).
		Where(`"student_data_change_request".target = ?`, target).
		Where(`"student_data_change_request".field_key = ?`, fieldKey).
		Where(`"student_data_change_request".status = ?`, users.DataChangeStatusPending)

	query = base.WithTenantFilter(ctx, query, "student_data_change_request")

	exists, err := query.Exists(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "check pending student data change request", Err: err}
	}
	return exists, nil
}

// FindPendingByIDForUpdate locks a change request row for the current tenant
// and verifies it is still pending. The lock closes the staff-decision race
// where one reviewer could apply live changes after another reviewer already
// decided the row.
func (r *StudentDataChangeRequestRepository) FindPendingByIDForUpdate(ctx context.Context, id int64) (*users.StudentDataChangeRequest, error) {
	row := new(users.StudentDataChangeRequest)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(row).
		ModelTableExpr(tableExprStudentDataChangeRequestsAsReq).
		Where(`"student_data_change_request".id = ?`, id).
		For("UPDATE")

	query = base.WithTenantFilter(ctx, query, "student_data_change_request")

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrChangeRequestNotFound
		}
		return nil, &modelBase.DatabaseError{Op: "find pending student data change request for update", Err: err}
	}
	if row.Status != users.DataChangeStatusPending {
		return nil, ErrChangeRequestNotPending
	}
	return row, nil
}

// Decide transitions a pending row to its final state. reviewed_at is stamped
// to now; applied_at is stamped only when the change was actually written to the
// live record (approvals). reviewedBy <= 0 leaves the reviewer NULL.
// UpdatePending rewrites a pending request's proposed value — the guardian
// edit path (#2267). old_value is deliberately untouched: it is the baseline
// the request was filed against, and staff compare the live value with it.
func (r *StudentDataChangeRequestRepository) UpdatePending(ctx context.Context, id int64, newValue json.RawMessage) error {
	q := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*users.StudentDataChangeRequest)(nil)).
		ModelTableExpr(tableExprStudentDataChangeRequestsAsReq).
		Set("new_value = ?", newValue).
		Set("updated_at = ?", time.Now()).
		Where(`"student_data_change_request".id = ?`, id).
		Where(`"student_data_change_request".status = ?`, users.DataChangeStatusPending)
	q = base.WithTenantFilter(ctx, q, "student_data_change_request")
	res, err := q.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update pending student data change request", Err: err}
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrChangeRequestNotPending
	}
	return nil
}

func (r *StudentDataChangeRequestRepository) Decide(ctx context.Context, id int64, newStatus string, reason *string, reviewedBy int64, applied bool) error {
	now := time.Now()
	q := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*users.StudentDataChangeRequest)(nil)).
		ModelTableExpr(tableExprStudentDataChangeRequestsAsReq).
		Set("status = ?", newStatus).
		Set("review_reason = ?", reason).
		Set("reviewed_at = ?", now).
		Set("updated_at = ?", now).
		Where(`"student_data_change_request".id = ?`, id).
		Where(`"student_data_change_request".status = ?`, users.DataChangeStatusPending)

	if reviewedBy > 0 {
		q = q.Set("reviewed_by = ?", reviewedBy)
	} else {
		q = q.Set("reviewed_by = NULL")
	}
	if applied {
		q = q.Set("applied_at = ?", now)
	}
	q = base.WithTenantFilter(ctx, q, "student_data_change_request")

	res, err := q.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "decide student data change request", Err: err}
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		// No pending row with this id under the current tenant — already
		// decided by another reviewer, or not found.
		return ErrChangeRequestNotPending
	}
	return nil
}

// FindByIDForUpdate locks a request row whatever its status. The correction
// path starts from a decided row, which FindPendingByIDForUpdate refuses by
// design.
func (r *StudentDataChangeRequestRepository) FindByIDForUpdate(ctx context.Context, id int64) (*users.StudentDataChangeRequest, error) {
	row := new(users.StudentDataChangeRequest)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(row).
		ModelTableExpr(tableExprStudentDataChangeRequestsAsReq).
		Where(`"student_data_change_request".id = ?`, id).
		For("UPDATE")
	query = base.WithTenantFilter(ctx, query, "student_data_change_request")
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrChangeRequestNotFound
		}
		return nil, &modelBase.DatabaseError{Op: "find student data change request for update", Err: err}
	}
	return row, nil
}

// Redecide rewrites an already decided row. The WHERE clause names the two
// states a correction may start from, so a concurrent care-end close or a
// second correction cannot be silently overwritten by one prepared earlier.
func (r *StudentDataChangeRequestRepository) Redecide(
	ctx context.Context, id int64, newStatus string, reason *string, reviewedBy int64, applied bool,
) error {
	now := time.Now()
	q := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*users.StudentDataChangeRequest)(nil)).
		ModelTableExpr(tableExprStudentDataChangeRequestsAsReq).
		Set("status = ?", newStatus).
		Set("review_reason = ?", reason).
		Set("reviewed_by = ?", reviewedBy).
		Set("reviewed_at = ?", now).
		Set("updated_at = ?", now).
		Where(`"student_data_change_request".id = ?`, id).
		Where(`"student_data_change_request".status IN (?)`, bun.List([]string{
			users.DataChangeStatusApproved,
			users.DataChangeStatusRejected,
		}))
	if applied {
		q = q.Set("applied_at = ?", now)
	} else {
		q = q.Set("applied_at = NULL")
	}
	q = base.WithTenantFilter(ctx, q, "student_data_change_request")
	res, err := q.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "correct student data change request", Err: err}
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrChangeRequestNotDecided
	}
	return nil
}
