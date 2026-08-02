package enrollment

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

const requestChildTableExpr = `enrollment.request_children AS "request_child"`

type RequestChildRepository struct {
	db *bun.DB
}

func NewRequestChildRepository(db *bun.DB) enrollment.RequestChildRepository {
	return &RequestChildRepository{db: db}
}

func (r *RequestChildRepository) Create(ctx context.Context, child *enrollment.RequestChild) error {
	base.EnsureTenantID(ctx, child)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(child).
		ModelTableExpr(requestChildTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create request child: %w", err)
	}
	return nil
}

// FindByID looks up one request_child by primary key. Tenant-scoped
// via RLS (the row is visible only when the current tenant context
// matches the row's tenant_id, or under an admin tx).
func (r *RequestChildRepository) FindByID(ctx context.Context, id int64) (*enrollment.RequestChild, error) {
	child := new(enrollment.RequestChild)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(child).
		ModelTableExpr(requestChildTableExpr).
		Where(`"request_child".id = ?`, id).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find request child %d: %w", id, err)
	}
	return child, nil
}

func (r *RequestChildRepository) ListByIDs(ctx context.Context, ids []int64) ([]*enrollment.RequestChild, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var children []*enrollment.RequestChild
	if err := base.GetDB(ctx, r.db).NewSelect().
		Model(&children).
		ModelTableExpr(requestChildTableExpr).
		Where(`"request_child".id IN (?)`, bun.List(ids)).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list request children by ids: %w", err)
	}
	return children, nil
}

// ListByRequestID returns all children for a request, sorted by sort_order.
func (r *RequestChildRepository) ListByRequestID(ctx context.Context, requestID int64) ([]*enrollment.RequestChild, error) {
	return r.listByRequestID(ctx, requestID, "")
}

func (r *RequestChildRepository) ListByRequestIDForUpdate(ctx context.Context, requestID int64) ([]*enrollment.RequestChild, error) {
	return r.listByRequestID(ctx, requestID, "UPDATE")
}

func (r *RequestChildRepository) listByRequestID(ctx context.Context, requestID int64, lockClause string) ([]*enrollment.RequestChild, error) {
	var children []*enrollment.RequestChild
	q := base.GetDB(ctx, r.db).NewSelect().
		Model(&children).
		ModelTableExpr(requestChildTableExpr).
		Where(`"request_child".request_id = ?`, requestID).
		OrderExpr(`"request_child".sort_order, "request_child".id`)
	if lockClause != "" {
		q = q.For(lockClause)
	}
	err := q.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request children: %w", err)
	}
	return children, nil
}

// ListByRequestIDs returns every child across the given requests in a
// single query, sorted by request_id, sort_order, id so the caller can
// group rows by request without re-sorting. Tenant-scoped via RLS.
// Empty input short-circuits to an empty slice (no query, no IN ()).
func (r *RequestChildRepository) ListByRequestIDs(ctx context.Context, requestIDs []int64) ([]*enrollment.RequestChild, error) {
	if len(requestIDs) == 0 {
		return nil, nil
	}
	var children []*enrollment.RequestChild
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&children).
		ModelTableExpr(requestChildTableExpr).
		Where(`"request_child".request_id IN (?)`, bun.List(requestIDs)).
		OrderExpr(`"request_child".request_id, "request_child".sort_order, "request_child".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request children by request ids: %w", err)
	}
	return children, nil
}

// UpdateStatus transitions a single child to a new status. PR 8's
// decision service is the primary caller; PR 5 ships this so the schema
// and shape are pinned.
func (r *RequestChildRepository) UpdateStatus(ctx context.Context, id int64, newStatus string, reason *string, reviewedBy int64) error {
	now := time.Now()
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.RequestChild)(nil)).
		ModelTableExpr(requestChildTableExpr).
		Set("status = ?", newStatus).
		Set("status_reason = ?", reason).
		Set("reviewed_at = ?", now).
		Where(`"request_child".id = ?`, id)
	if reviewedBy > 0 {
		q = q.Set("reviewed_by = ?", reviewedBy)
	} else {
		// Parent-initiated transitions (e.g., self-withdraw) carry no
		// reviewer. Setting NULL avoids a FK violation against
		// auth.accounts(id).
		q = q.Set("reviewed_by = NULL")
	}

	res, err := q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request child status: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("request child %d not found", id)
	}
	return nil
}

// UpdateData updates the parent-supplied child fields without changing
// status, review metadata, rollover metadata, or created_student_id.
func (r *RequestChildRepository) UpdateData(ctx context.Context, child *enrollment.RequestChild) error {
	if child == nil || child.ID <= 0 {
		return fmt.Errorf("request child id is required")
	}
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(child).
		ModelTableExpr(requestChildTableExpr).
		Set("first_name = ?", child.FirstName).
		Set("last_name = ?", child.LastName).
		Set("date_of_birth = ?", child.DateOfBirth).
		Set("target_grade_level = ?", child.TargetGradeLevel).
		Set("target_school_class = ?", child.TargetSchoolClass).
		Set("custom_data = ?", child.CustomData).
		Set("sort_order = ?", child.SortOrder).
		Set("updated_at = NOW()").
		Where(`"request_child".id = ?`, child.ID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request child data: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("request child %d not found", child.ID)
	}
	return nil
}

// LinkCreatedStudent stamps the request_children row with the id of the
// student created on approval, so the admin UI can navigate from a
// historical request to the resulting student profile.
func (r *RequestChildRepository) LinkCreatedStudent(ctx context.Context, requestChildID, studentID int64) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.RequestChild)(nil)).
		ModelTableExpr(requestChildTableExpr).
		Set("created_student_id = ?", studentID).
		Where(`"request_child".id = ?`, requestChildID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to link created student: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("request child %d not found", requestChildID)
	}
	return nil
}

// UpdateMatchedStudent re-points (or clears) the already-enrolled student an
// existing_students child is pinned to. Submission and replacement edits write
// the pin as part of the INSERT; the change-request path edits an already
// persisted row in place, so an approved identity change needs this to keep the
// pin describing the same human the row now names (#1663). A nil studentID
// clears the pin, which sends approval down the fresh-create branch.
func (r *RequestChildRepository) UpdateMatchedStudent(ctx context.Context, requestChildID int64, studentID *int64) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.RequestChild)(nil)).
		ModelTableExpr(requestChildTableExpr).
		Set("matched_student_id = ?", studentID).
		Set("updated_at = ?", time.Now()).
		Where(`"request_child".id = ?`, requestChildID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request child matched student: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("request child %d not found", requestChildID)
	}
	return nil
}

// UpdateActivationPlan records the approval-time lifecycle decision on
// the child row. Submission rows start with the schema default; approval is
// the first point where the tenant setting and phase dates are authoritative.
func (r *RequestChildRepository) UpdateActivationPlan(ctx context.Context, requestChildID int64, mode string, activateOn *timezone.Date) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.RequestChild)(nil)).
		ModelTableExpr(requestChildTableExpr).
		Set("activation_mode = ?", mode).
		Set("activate_on = ?", activateOn).
		Set("updated_at = ?", time.Now()).
		Where(`"request_child".id = ?`, requestChildID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request child activation plan: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("request child %d not found", requestChildID)
	}
	return nil
}

// DeleteByRequestID removes every child row under the request. The
// request_child_offerings FK is ON DELETE CASCADE, so per-child offering links
// are removed in the same statement. Used by the parent edit replace flow
// after it has locked and verified the children are still editable.
func (r *RequestChildRepository) DeleteByRequestID(ctx context.Context, requestID int64) error {
	if requestID <= 0 {
		return fmt.Errorf("request id must be positive")
	}
	_, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*enrollment.RequestChild)(nil)).
		ModelTableExpr(requestChildTableExpr).
		Where(`"request_child".request_id = ?`, requestID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete request children: %w", err)
	}
	return nil
}

// CountCreatedStudentsByPhaseID returns the number of distinct students
// that were created from the phase's enrollment requests (children with
// a non-null created_student_id). Powers the phase-delete confirmation
// modal ("Z bereits angelegte Kinder bleiben erhalten"). Those students
// survive the phase delete — created_student_id is ON DELETE SET NULL.
// Tenant-scoped via RLS on both tables.
func (r *RequestChildRepository) CountCreatedStudentsByPhaseID(ctx context.Context, phaseID int64) (int, error) {
	if phaseID <= 0 {
		return 0, fmt.Errorf("phase id must be positive")
	}
	var count int
	err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(requestChildTableExpr).
		Join(`INNER JOIN enrollment.requests AS "request" ON "request".id = "request_child".request_id`).
		Where(`"request".phase_id = ?`, phaseID).
		Where(`"request_child".created_student_id IS NOT NULL`).
		ColumnExpr(`COUNT(DISTINCT "request_child".created_student_id)`).
		Scan(ctx, &count)
	if err != nil {
		return 0, fmt.Errorf("failed to count created students by phase: %w", err)
	}
	return count, nil
}

// ListCarePeriodsByStudentID returns the approved enrollments behind a student
// together with the care window of their phase, latest window first. Joined
// rather than filtered because the window lives on enrollment.phases, two joins
// away from the child row. Tenant RLS applies on all three tables.
func (r *RequestChildRepository) ListCarePeriodsByStudentID(
	ctx context.Context,
	studentID int64,
) ([]*enrollment.StudentCarePeriod, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("student id must be positive")
	}
	periods := make([]*enrollment.StudentCarePeriod, 0)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&periods).
		ModelTableExpr(requestChildTableExpr).
		Join(`INNER JOIN enrollment.requests AS "request" ON "request".id = "request_child".request_id`).
		Join(`INNER JOIN enrollment.phases AS "phase" ON "phase".id = "request".phase_id`).
		ColumnExpr(`"request_child".id AS request_child_id`).
		ColumnExpr(`"request".id AS request_id`).
		ColumnExpr(`"phase".id AS phase_id`).
		ColumnExpr(`"phase".name AS phase_name`).
		ColumnExpr(`"phase".service_start_date AS service_start_date`).
		ColumnExpr(`"phase".service_end_date AS service_end_date`).
		Where(`"request_child".created_student_id = ?`, studentID).
		Where(`"request_child".status = ?`, enrollment.ChildStatusApproved).
		OrderExpr(`"phase".service_start_date DESC, "request_child".id DESC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list care periods by student: %w", err)
	}
	return periods, nil
}

// ListByPhaseAndStatuses joins through enrollment.requests to filter by
// phase_id, then narrows by status set. Tenant RLS on both tables.
func (r *RequestChildRepository) ListByPhaseAndStatuses(ctx context.Context, phaseID int64, statuses []string) ([]*enrollment.RequestChild, error) {
	if phaseID <= 0 {
		return nil, fmt.Errorf("phase id must be positive")
	}
	if len(statuses) == 0 {
		return nil, nil
	}
	var children []*enrollment.RequestChild
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&children).
		ModelTableExpr(requestChildTableExpr).
		Join(`INNER JOIN enrollment.requests AS "request" ON "request".id = "request_child".request_id`).
		Where(`"request".phase_id = ?`, phaseID).
		Where(`"request_child".status IN (?)`, bun.List(statuses)).
		OrderExpr(`"request_child".request_id, "request_child".sort_order, "request_child".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request children by phase + status: %w", err)
	}
	return children, nil
}

// BulkUpdateStatusByPhaseAndStatus drives the deadline worker. One
// UPDATE per (phase, current_status) pair; returns RowsAffected so the
// caller can report how many rows transitioned.
func (r *RequestChildRepository) BulkUpdateStatusByPhaseAndStatus(ctx context.Context, phaseID int64, currentStatus, newStatus string) (int, error) {
	if phaseID <= 0 {
		return 0, fmt.Errorf("phase id must be positive")
	}
	if currentStatus == "" || newStatus == "" {
		return 0, fmt.Errorf("current and new status are required")
	}

	res, err := base.GetDB(ctx, r.db).NewRaw(`
		UPDATE enrollment.request_children AS rc
		SET status = ?, updated_at = NOW()
		FROM enrollment.requests AS r
		WHERE r.id = rc.request_id
		  AND r.phase_id = ?
		  AND rc.status = ?
	`, newStatus, phaseID, currentStatus).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("bulk update status: %w", err)
	}
	rows, _ := res.RowsAffected()
	return int(rows), nil
}

// UpdateRolloverReview transitions an admin-review row to its final
// status. When newGradeLevel is non-nil, target_grade_level is also
// rewritten — used by the "Behalten" action to keep a student at a
// repeated grade.
func (r *RequestChildRepository) UpdateRolloverReview(ctx context.Context, id int64, newStatus string, reason *string, newGradeLevel *int16, reviewedBy int64) error {
	now := time.Now()
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.RequestChild)(nil)).
		ModelTableExpr(requestChildTableExpr).
		Set("status = ?", newStatus).
		Set("status_reason = ?", reason).
		Set("reviewed_at = ?", now).
		Where(`"request_child".id = ?`, id)
	if newGradeLevel != nil {
		q = q.Set("target_grade_level = ?", *newGradeLevel)
	}
	if reviewedBy > 0 {
		q = q.Set("reviewed_by = ?", reviewedBy)
	}
	// Clear review_reason once the row leaves admin review.
	q = q.Set("review_reason = NULL")

	res, err := q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update rollover review: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("request child %d not found", id)
	}
	return nil
}
