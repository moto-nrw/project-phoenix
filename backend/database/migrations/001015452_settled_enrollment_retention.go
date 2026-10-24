package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	settledEnrollmentRetentionVersion     = "1.15.452"
	settledEnrollmentRetentionDescription = "Zurückgezogene und verfallene Anmeldungen ohne Kind nach der Löschfrist der Schule löschen (#3960)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     settledEnrollmentRetentionVersion,
		Description: settledEnrollmentRetentionDescription,
		DependsOn:   []string{cleanupJobRetentionVersion}, // previous subject check of audit.data_deletions
	})
	Migrations.MustRegister(settledEnrollmentRetentionUp, settledEnrollmentRetentionDown)
}

// settledEnrollmentRetentionKind is the data kind of the deletion log this
// migration adds.
const settledEnrollmentRetentionKind = "settled_enrollment_retention"

// settledEnrollmentRetentionUpSQL extends the subject check of 1.15.449 by
// the settled enrollment kind. The tenant role already deletes enrollment
// requests and late invites for the cleanup of rejected enrollments, and RLS
// keeps the nightly run to its school, so the deletion log is the only
// change.
const settledEnrollmentRetentionUpSQL = `
		ALTER TABLE audit.data_deletions
			DROP CONSTRAINT IF EXISTS chk_data_deletions_subject;
		ALTER TABLE audit.data_deletions
			ADD CONSTRAINT chk_data_deletions_subject CHECK (
				(student_id IS NULL OR staff_id IS NULL)
				AND (
					student_id IS NOT NULL
					OR staff_id IS NOT NULL
					OR deletion_type IN (
						'unregistered_tag_scan_retention',
						'status_day_retention',
						'parent_message_retention',
						'auth_event_retention',
						'data_access_log_retention',
						'deletion_log_retention',
						'delivery_retention',
						'file_cleanup_retention',
						'staff_cleanup_retention',
						'student_document_cleanup_retention',
						'care_withdrawal_retention',
						'submission_rate_limit_retention',
						'settled_enrollment_retention'
					)
				)
			);
`

// settledEnrollmentRetentionDownSQL restores the subject check of 1.15.449.
const settledEnrollmentRetentionDownSQL = `
		ALTER TABLE audit.data_deletions
			DROP CONSTRAINT IF EXISTS chk_data_deletions_subject;
		ALTER TABLE audit.data_deletions
			ADD CONSTRAINT chk_data_deletions_subject CHECK (
				(student_id IS NULL OR staff_id IS NULL)
				AND (
					student_id IS NOT NULL
					OR staff_id IS NOT NULL
					OR deletion_type IN (
						'unregistered_tag_scan_retention',
						'status_day_retention',
						'parent_message_retention',
						'auth_event_retention',
						'data_access_log_retention',
						'deletion_log_retention',
						'delivery_retention',
						'file_cleanup_retention',
						'staff_cleanup_retention',
						'student_document_cleanup_retention',
						'care_withdrawal_retention',
						'submission_rate_limit_retention'
					)
				)
			);
`

func settledEnrollmentRetentionUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()
	if _, err := tx.ExecContext(ctx, settledEnrollmentRetentionUpSQL); err != nil {
		return fmt.Errorf("allow settled enrollment retention in the deletion log: %w", err)
	}
	return tx.Commit()
}

func settledEnrollmentRetentionDown(ctx context.Context, db *bun.DB) error {
	var logged int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.data_deletions
		WHERE deletion_type = ? AND student_id IS NULL AND staff_id IS NULL`, settledEnrollmentRetentionKind).Scan(ctx, &logged); err != nil {
		return fmt.Errorf("check settled enrollment retention deletion log rows: %w", err)
	}
	if logged > 0 {
		return fmt.Errorf("cannot restore the data_deletions subject check while %d settled enrollment retention rows exist", logged)
	}
	if _, err := db.ExecContext(ctx, settledEnrollmentRetentionDownSQL); err != nil {
		return fmt.Errorf("restore the data_deletions subject check: %w", err)
	}
	return nil
}
