package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

// SourceEnrollmentCount is the timetable legacy view's count of the
// enrollments a request (childID nil) or one of its children sourced. The
// root binds it; Enrollment never reads activities.student_enrollments.
type SourceEnrollmentCount func(ctx context.Context, db bun.IDB, tenantID, requestID int64, childID *int64) (int, error)

// SourceEnrollments resolves rows on the caller's existing unit of work.
func (r *Store) SourceEnrollments(count SourceEnrollmentCount) func(context.Context, int64, *int64) (int, error) {
	return func(ctx context.Context, requestID int64, childID *int64) (int, error) {
		if count == nil {
			return 0, errors.New("enrollment postgres: source enrollment count is required")
		}
		tenantID, err := r.tenantID(ctx)
		if err != nil {
			return 0, err
		}
		db, err := r.resolve(ctx)
		if err != nil {
			return 0, err
		}
		return count(ctx, db, tenantID, requestID, childID)
	}
}

func (r *Store) DeletionRequestCounts(ctx context.Context, requestID int64) (*enrollment.DeletionRequestCounts, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	row := new(enrollment.DeletionRequestCounts)
	err = db.NewRaw(`
		WITH target_request AS (
			SELECT id, guardian_account_id
			FROM enrollment.requests
			WHERE id = ? AND tenant_id = ?
		), target_children AS (
			SELECT id
			FROM enrollment.request_children
			WHERE request_id = ? AND tenant_id = ?
		), target_changes AS (
			SELECT id
			FROM enrollment.change_requests
			WHERE request_id = ? AND tenant_id = ?
		)
		SELECT
			(SELECT COUNT(*) FROM target_request)::int AS requests,
			(SELECT guardian_account_id FROM target_request LIMIT 1) AS guardian_account_id,
			(SELECT COUNT(*) FROM target_children)::int AS request_children,
			(SELECT COUNT(*) FROM enrollment.request_guardians g WHERE g.request_id = ? AND g.tenant_id = ?)::int AS request_guardians,
			(SELECT COUNT(*) FROM target_changes)::int AS change_requests,
			(SELECT COUNT(*) FROM enrollment.change_request_messages m JOIN target_changes c ON c.id = m.change_request_id WHERE m.tenant_id = ?)::int AS change_request_messages,
			(SELECT COUNT(*) FROM enrollment.late_invites l WHERE l.used_request_id = ? AND l.tenant_id = ?)::int AS late_invites,
			0::int AS email_outbox,
			(SELECT COUNT(*) FROM enrollment.request_children c WHERE c.rollover_source_child_id IN (SELECT id FROM target_children) AND c.tenant_id = ?)::int AS rollover_links_cleared,
			0::int AS student_source_links_cleared
	`,
		requestID, tenantID,
		requestID, tenantID,
		requestID, tenantID,
		requestID, tenantID,
		tenantID,
		requestID, tenantID,
		tenantID,
	).Scan(ctx, row)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment request deletion: %w", err)
	}
	return row, nil
}
func (r *Store) DeletionChildTarget(ctx context.Context, requestID, childID int64) (*enrollment.DeletionChildTarget, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	row := new(enrollment.DeletionChildTarget)
	err = db.NewRaw(`
		SELECT
			COUNT(*) FILTER (WHERE id = ?)::int AS target_children,
			COUNT(*)::int AS all_children
		FROM enrollment.request_children
		WHERE request_id = ? AND tenant_id = ?
	`, childID, requestID, tenantID).Scan(ctx, row)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment child deletion target: %w", err)
	}
	return row, nil
}
func (r *Store) DeletionChildCounts(ctx context.Context, requestID, childID int64) (*enrollment.DeletionChildCounts, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	row := new(enrollment.DeletionChildCounts)
	err = db.NewRaw(`
		WITH target_child AS (
			SELECT id FROM enrollment.request_children
			WHERE request_id = ? AND id = ? AND tenant_id = ?
		), target_changes AS (
			SELECT id FROM enrollment.change_requests
			WHERE request_id = ? AND request_child_id IN (SELECT id FROM target_child) AND tenant_id = ?
		)
		SELECT
			(SELECT COUNT(*) FROM target_changes)::int AS change_requests,
			(SELECT COUNT(*) FROM enrollment.change_request_messages m JOIN target_changes c ON c.id = m.change_request_id WHERE m.tenant_id = ?)::int AS change_request_messages,
			(SELECT COUNT(*) FROM enrollment.request_children WHERE rollover_source_child_id IN (SELECT id FROM target_child) AND tenant_id = ?)::int AS rollover_links,
			0::int AS student_source_links
	`, requestID, childID, tenantID, requestID, tenantID, tenantID, tenantID).Scan(ctx, row)
	if err != nil {
		return nil, fmt.Errorf("preview enrollment child deletion: %w", err)
	}
	return row, nil
}
func (r *Store) DeletionGuardianProfileIDs(ctx context.Context, requestID int64) ([]int64, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var row []int64
	err = db.NewRaw(`
		SELECT guardian_profile_id
		FROM enrollment.request_guardians
		WHERE request_id = ? AND tenant_id = ? AND guardian_profile_id IS NOT NULL
	`, requestID, tenantID).Scan(ctx, &row)
	if err != nil {
		return nil, fmt.Errorf("list request guardian profiles: %w", err)
	}
	return row, nil
}
func (r *Store) DeletionBlockingStudentIDs(ctx context.Context, requestID int64, childID *int64) ([]int64, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0)
	query := db.NewSelect().
		TableExpr(`enrollment.request_children AS "request_child"`).
		ColumnExpr(`"request_child".created_student_id`).
		Where(`"request_child".request_id = ?`, requestID).
		Where(`"request_child".tenant_id = ?`, tenantID).
		Where(`"request_child".created_student_id IS NOT NULL`)
	if childID != nil {
		query = query.Where(`"request_child".id = ?`, *childID)
	}
	if err := query.OrderExpr(`"request_child".created_student_id`).Scan(ctx, &ids); err != nil {
		return nil, fmt.Errorf("list students blocking enrollment deletion: %w", err)
	}
	return ids, nil
}
