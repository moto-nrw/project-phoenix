package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	return r.GetPendingForStudentAndKind(ctx, studentID, schedule.CareRequestKindWeeklySchedule)
}

func (r *CareScheduleChangeRequestRepository) GetPendingForStudentAndKind(ctx context.Context, studentID int64, requestKind string) (*schedule.CareScheduleChangeRequest, error) {
	row := new(schedule.CareScheduleChangeRequest)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(row).
		ModelTableExpr(tableExprCareScheduleChangeRequestsAsReq).
		Where(`"care_schedule_change_request".student_id = ?`, studentID).
		Where(`"care_schedule_change_request".request_kind = ?`, requestKind).
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

// ListPendingForTenant returns the pending requests of BOTH kinds (weekly plan
// and single-day pickup change) for the current tenant, newest submission
// first, narrowed and paged by filters. One query rather than one per kind:
// a keyset page has to be cut across the whole queue, not per kind.
func (r *CareScheduleChangeRequestRepository) ListPendingForTenant(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*schedule.CareScheduleChangeRequest, error) {
	return r.listPending(ctx, nil, filters)
}

// ListPendingForTenantAndKind is ListPendingForTenant narrowed to one request
// kind.
func (r *CareScheduleChangeRequestRepository) ListPendingForTenantAndKind(ctx context.Context, requestKind string, filters modelBase.RequestQueueFilters) ([]*schedule.CareScheduleChangeRequest, error) {
	return r.listPending(ctx, []string{requestKind}, filters)
}

func (r *CareScheduleChangeRequestRepository) listPending(ctx context.Context, kinds []string, filters modelBase.RequestQueueFilters) ([]*schedule.CareScheduleChangeRequest, error) {
	var rows []*schedule.CareScheduleChangeRequest
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprCareScheduleChangeRequestsAsReq).
		Where(`"care_schedule_change_request".status = ?`, schedule.CareRequestStatusPending)
	if len(kinds) > 0 {
		query = query.Where(`"care_schedule_change_request".request_kind IN (?)`, bun.List(kinds))
	}

	query = base.WithTenantFilter(ctx, query, "care_schedule_change_request")
	weekdayJSON := fmt.Sprintf(`[{"weekday":%d}]`, urgencyWeekday(filters.UrgentDate))
	query = base.ApplyRequestUrgency(query, filters, `
		("care_schedule_change_request".request_kind = 'pickup_change'
			AND "care_schedule_change_request".payload->>'date' = ?)
		OR ("care_schedule_change_request".request_kind <> 'pickup_change'
			AND "care_schedule_change_request".payload->'weekdays' @> ?::jsonb)
	`, filters.UrgentDate, weekdayJSON)
	query = base.ApplyRequestQueueFilters(query, "care_schedule_change_request", "created_at", filters)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list pending care schedule change requests", Err: err}
	}
	return rows, nil
}

func urgencyWeekday(raw string) int {
	date, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return 0
	}
	weekday := int(date.Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}

// ListDecidedForTenant returns the tenant's decided care-schedule requests
// (approved, rejected, withdrawn) newest-decision-first via keyset pagination
// on (updated_at, id). Every Decide stamps updated_at and decided rows are
// terminal, so it is the decision instant (withdrawn rows carry no
// reviewed_at). A zero BeforeInstant returns the first page.
func (r *CareScheduleChangeRequestRepository) ListDecidedForTenant(ctx context.Context, filters modelBase.RequestQueueFilters) ([]*schedule.CareScheduleChangeRequest, error) {
	var rows []*schedule.CareScheduleChangeRequest
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprCareScheduleChangeRequestsAsReq).
		Where(`"care_schedule_change_request".status IN (?)`, bun.List([]string{
			schedule.CareRequestStatusApproved,
			schedule.CareRequestStatusRejected,
			schedule.CareRequestStatusWithdrawn,
		}))

	query = base.WithTenantFilter(ctx, query, "care_schedule_change_request")
	query = base.ApplyRequestQueueFilters(query, "care_schedule_change_request", "updated_at", filters)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list decided care schedule change requests", Err: err}
	}
	return rows, nil
}

func (r *CareScheduleChangeRequestRepository) ListRecentForStudentAndKind(ctx context.Context, studentID int64, requestKind string, since time.Time) ([]*schedule.CareScheduleChangeRequest, error) {
	var rows []*schedule.CareScheduleChangeRequest
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprCareScheduleChangeRequestsAsReq).
		Where(`"care_schedule_change_request".student_id = ?`, studentID).
		Where(`"care_schedule_change_request".request_kind = ?`, requestKind).
		Where(`("care_schedule_change_request".status = ? OR "care_schedule_change_request".updated_at >= ?)`, schedule.CareRequestStatusPending, since)
	query = base.WithTenantFilter(ctx, query, "care_schedule_change_request")
	query = query.OrderExpr(`"care_schedule_change_request".created_at DESC`).OrderExpr(`"care_schedule_change_request".id DESC`)
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list recent care schedule change requests", Err: err}
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

// UpdateDecisionSnapshot stores the frozen review diff on a decided row (ADR
// 0002, #2430). Separate from Decide so the race-guarded transition keeps its
// signature; callers write the snapshot in the same transaction.
func (r *CareScheduleChangeRequestRepository) UpdateDecisionSnapshot(ctx context.Context, id int64, snapshot *schedule.CareRequestDecisionSnapshot) error {
	q := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*schedule.CareScheduleChangeRequest)(nil)).
		ModelTableExpr(tableExprCareScheduleChangeRequestsAsReq).
		Set("decision_snapshot = ?", snapshot).
		Set("updated_at = ?", time.Now()).
		Where(`"care_schedule_change_request".id = ?`, id)
	q = base.WithTenantFilter(ctx, q, "care_schedule_change_request")
	res, err := q.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update care request decision snapshot", Err: err}
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrCareRequestNotFound
	}
	return nil
}
