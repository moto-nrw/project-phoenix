package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

const (
	withdrawalStatePending   = "pending"
	withdrawalStateResolved  = "resolved"
	withdrawalOutcomeDeleted = "deleted"
)

// FindPendingWithdrawalStudent resolves the child a pending withdrawal task
// names. A resolved, obsolete or already redacted task is reported as not
// pending; lock takes the task row FOR UPDATE.
func (s *Store) FindPendingWithdrawalStudent(ctx context.Context, completionID int64, lock bool) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	var row struct {
		StudentID *int64 `bun:"student_id"`
		State     string `bun:"state"`
	}
	query := db.NewSelect().TableExpr(`users.care_withdrawal_completions AS "completion"`).
		ColumnExpr(`"completion".student_id, "completion".state`).
		Where(`"completion".tenant_id = ?`, tenantID).
		Where(`"completion".id = ?`, completionID)
	if lock {
		query = query.For("UPDATE")
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &row)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, stats, domain.ErrWithdrawalNotFound
	}
	if err != nil {
		return 0, stats, fmt.Errorf("care plan postgres: find pending withdrawal: %w", err)
	}
	stats.Rows = 1
	if row.State != withdrawalStatePending || row.StudentID == nil || *row.StudentID <= 0 {
		return 0, stats, domain.ErrWithdrawalNotPending
	}
	return *row.StudentID, stats, nil
}

// ResolvePendingWithdrawalAsDeleted closes one pending task with the deleted
// outcome and removes its child references. False means the task was no
// longer pending.
func (s *Store) ResolvePendingWithdrawalAsDeleted(ctx context.Context, completionID, actorAccountID int64, at time.Time) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "resolve withdrawal as deleted")
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	stats, rows, err := execute(ctx, db.NewUpdate().TableExpr(`users.care_withdrawal_completions AS "completion"`).
		Set(`state = ?`, withdrawalStateResolved).
		Set(`outcome = ?`, withdrawalOutcomeDeleted).
		Set(`student_id = NULL`).
		Set(`source_adjustment_id = NULL`).
		Set(`source_request_child_id = NULL`).
		Set(`source_offerings = '[]'::jsonb`).
		Set(`obsolete_reason = NULL`).
		Set(`resolved_by = ?`, actorAccountID).
		Set(`resolved_at = ?`, at).
		Set(`updated_at = ?`, at).
		Where(`"completion".tenant_id = ?`, tenantID).
		Where(`"completion".id = ?`, completionID).
		Where(`"completion".state = ?`, withdrawalStatePending), "resolve withdrawal as deleted")
	if err != nil {
		return false, stats, err
	}
	return rows == 1, stats, nil
}

// RedactWithdrawalsForDeletedStudent redacts every task still linked to the
// child: a pending task is resolved with the deleted outcome, a finished one
// only loses its child references. It returns the number of redacted rows.
func (s *Store) RedactWithdrawalsForDeletedStudent(ctx context.Context, studentID, actorAccountID int64, at time.Time) (int, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "redact withdrawals for deleted student")
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats, rows, err := execute(ctx, db.NewUpdate().TableExpr(`users.care_withdrawal_completions AS "completion"`).
		Set(`state = CASE WHEN state = ? THEN ? ELSE state END`, withdrawalStatePending, withdrawalStateResolved).
		Set(`outcome = CASE WHEN state = ? THEN ? ELSE outcome END`, withdrawalStatePending, withdrawalOutcomeDeleted).
		Set(`student_id = NULL`).
		Set(`source_adjustment_id = NULL`).
		Set(`source_request_child_id = NULL`).
		Set(`source_offerings = '[]'::jsonb`).
		Set(`obsolete_reason = CASE WHEN state = ? THEN NULL ELSE obsolete_reason END`, withdrawalStatePending).
		Set(`resolved_by = CASE WHEN state = ? THEN ? ELSE resolved_by END`, withdrawalStatePending, actorAccountID).
		Set(`resolved_at = CASE WHEN state = ? THEN ? ELSE resolved_at END`, withdrawalStatePending, at).
		Set(`updated_at = ?`, at).
		Where(`"completion".tenant_id = ?`, tenantID).
		Where(`"completion".student_id = ?`, studentID), "redact withdrawals for deleted student")
	if err != nil {
		return 0, stats, err
	}
	return int(rows), stats, nil
}
