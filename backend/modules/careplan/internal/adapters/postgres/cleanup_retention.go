package postgres

import (
	"context"
	"fmt"
	"time"
)

// DeleteFinishedStudentDocumentFileCleanups goes through the database
// function of 1.15.449: the direct DELETE on the intents stays revoked from
// the tenant role, and the function deletes only finished intents of the
// transaction's school. Its database clock supplies the cutoff, avoiding host clock skew.
func (s *Store) DeleteFinishedStudentDocumentFileCleanups(ctx context.Context, window time.Duration) (int, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "delete finished student document file cleanups")
	if err != nil {
		return 0, err
	}
	var deleted int64
	if err := db.NewRaw(`SELECT users.delete_finished_student_document_file_cleanups(?, statement_timestamp() - (? * INTERVAL '1 day'))`, tenantID, int(window/(24*time.Hour))).Scan(ctx, &deleted); err != nil {
		return 0, fmt.Errorf("care plan postgres: delete finished student document file cleanups: %w", err)
	}
	return int(deleted), nil
}

// DeleteFinishedWithdrawalCompletions removes resolved and obsolete
// completions; a pending one always stays. A care exit that named the
// completion loses only the link (ON DELETE SET NULL).
func (s *Store) DeleteFinishedWithdrawalCompletions(ctx context.Context, window time.Duration) (int, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "delete finished withdrawal completions")
	if err != nil {
		return 0, err
	}
	result, err := db.NewRaw(`DELETE FROM users.care_withdrawal_completions
		WHERE tenant_id = ? AND state <> ? AND resolved_at < statement_timestamp() - (? * INTERVAL '1 day')`, tenantID, withdrawalStatePending, int(window/(24*time.Hour))).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("care plan postgres: delete finished withdrawal completions: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("care plan postgres: delete finished withdrawal completions: %w", err)
	}
	return int(rows), nil
}
