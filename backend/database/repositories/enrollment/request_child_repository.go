package enrollment

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
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

// ListByRequestID returns all children for a request, sorted by sort_order.
func (r *RequestChildRepository) ListByRequestID(ctx context.Context, requestID int64) ([]*enrollment.RequestChild, error) {
	var children []*enrollment.RequestChild
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&children).
		ModelTableExpr(requestChildTableExpr).
		Where(`"request_child".request_id = ?`, requestID).
		OrderExpr(`"request_child".sort_order, "request_child".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request children: %w", err)
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

// CountCreatedStudentsByPhaseID returns the number of distinct students
// that were created from the phase's enrollment requests (children with
// a non-null created_student_id). Powers the phase-delete confirmation
// modal ("Z bereits angelegte Schüler bleiben erhalten"). Those students
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
