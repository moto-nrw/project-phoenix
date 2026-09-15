package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
)

// purgedStudentPlaceholder mirrors schoolstructure.PurgedStudentPlaceholder;
// the adapter keeps its own copy so it does not import the public package.
const purgedStudentPlaceholder = "Gelöschtes Kind"

func (s *Store) CountStudentTransitionHistory(ctx context.Context, tenantID, studentID int64) (int, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	var count int
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`SELECT COUNT(*)::int FROM education.grade_transition_history
		WHERE tenant_id = ? AND student_id = ? AND person_name <> ?`, tenantID, studentID, purgedStudentPlaceholder).Scan(ctx, &count)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("school structure postgres: count student transition history: %w", err)
	}
	stats.Rows = 1
	return count, stats, nil
}

func (s *Store) AnonymizeStudentTransitionHistory(ctx context.Context, tenantID, studentID int64) (int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().TableExpr(`education.grade_transition_history AS "history"`).
		Set(`person_name = ?`, purgedStudentPlaceholder).
		Set(`rfid_tag = NULL`).
		Set(`updated_at = NOW()`).
		Where(`"history".tenant_id = ?`, tenantID).
		Where(`"history".student_id = ?`, studentID).
		Where(`("history".person_name <> ? OR "history".rfid_tag IS NOT NULL)`, purgedStudentPlaceholder).
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("school structure postgres: anonymize student transition history: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("school structure postgres: anonymize student transition history: count rows: %w", err)
	}
	return stats.Rows, stats, nil
}
