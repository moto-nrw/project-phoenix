package postgres

import (
	"context"
	"fmt"
	"time"
)

// DeleteFinishedStaffDocumentFileCleanups goes through the database function
// of 1.15.449: the direct DELETE on the intents stays revoked from the
// tenant role, and the function deletes only finished intents of the
// transaction's school.
func (s *Store) DeleteFinishedStaffDocumentFileCleanups(ctx context.Context, window time.Duration) (int, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	var deleted int64
	if err := db.NewRaw(`SELECT users.delete_finished_staff_document_file_cleanups(?, statement_timestamp() - (? * INTERVAL '1 second'))`, scope.TenantID, window.Seconds()).Scan(ctx, &deleted); err != nil {
		return 0, fmt.Errorf("workforce postgres: delete finished staff document file cleanups: %w", err)
	}
	return int(deleted), nil
}

func (s *Store) DeleteCompletedOffboardingCleanups(ctx context.Context, window time.Duration) (int, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	result, err := db.NewRaw(`DELETE FROM users.staff_offboarding_cleanup
		WHERE tenant_id = ? AND completed_at IS NOT NULL AND completed_at < statement_timestamp() - (? * INTERVAL '1 second')`, scope.TenantID, window.Seconds()).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("workforce postgres: delete completed offboarding cleanups: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("workforce postgres: delete completed offboarding cleanups: %w", err)
	}
	return int(rows), nil
}

// DeleteStaffSickNotesUploadedBefore goes through the database function of
// 1.15.453: the tenant role has no DELETE on staff documents, and the
// function deletes only sick notes of the transaction's school and queues
// their files in the staff document file cleanup.
func (s *Store) DeleteStaffSickNotesUploadedBefore(ctx context.Context, before time.Time) (int, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	var deleted int64
	if err := db.NewRaw(`SELECT users.delete_expired_staff_sick_notes(?, ?)`, scope.TenantID, before).Scan(ctx, &deleted); err != nil {
		return 0, fmt.Errorf("workforce postgres: delete expired staff sick notes: %w", err)
	}
	return int(deleted), nil
}
