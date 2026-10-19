package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const schoolContentRetentionVersion = "1.15.465"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     schoolContentRetentionVersion,
		Description: "Inhalte der Schule nach der Löschfrist löschen: Elternmitteilungen, Team-Hinweise, Termine, Jahrgangswechsel, gelöschte Dateien (#4045)",
		DependsOn: []string{
			accountGuardianRetentionVersion, // previous subject check of audit.data_deletions; the last migration before this one
			cleanupJobRetentionVersion,      // the function pattern of the file cleanup intents
		},
	})
	Migrations.MustRegister(schoolContentRetentionUp, schoolContentRetentionDown)
}

// schoolContentRetentionKinds are the data kinds of the deletion log this
// migration adds.
var schoolContentRetentionKinds = []string{
	"parent_announcement_retention",
	"staff_notice_retention",
	"appointment_retention",
	"appointment_tombstone_retention",
	"grade_transition_retention",
	"removed_file_retention",
}

// schoolContentRetentionUpSQL lets the nightly retention of a school delete
// the content it created itself (#4045) and log each data kind as one row
// per run.
//
//   - Parent announcements, staff notices, appointments and grade
//     transitions: the tenant role already deletes them, and RLS keeps the
//     run to its school. Their child rows go through ON DELETE CASCADE.
//   - The file storage keeps the direct DELETE on documents.files revoked
//     from phoenix_tenant (1.15.332): a request removes a file only softly,
//     so its cleanup intent finds the bytes. A function in the pattern of
//     1.15.449 runs as its owner, accepts only the tenant of the calling
//     transaction, deletes only rows whose bytes are gone and refuses a
//     cutoff newer than 30 days.
const schoolContentRetentionUpSQL = `
		CREATE OR REPLACE FUNCTION documents.delete_removed_files(
			p_tenant_id BIGINT,
			p_cutoff TIMESTAMPTZ
		)
		RETURNS BIGINT
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog
		AS $$
		DECLARE
			deleted_count BIGINT;
			invoker_is_superuser BOOLEAN;
		BEGIN
			IF p_cutoff IS NULL THEN
				RAISE EXCEPTION 'removed file retention requires a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);
			IF NOT COALESCE(invoker_is_superuser, FALSE) AND p_cutoff > clock_timestamp() - INTERVAL '30 days' THEN
				RAISE EXCEPTION 'removed file cutoff exceeds retention window' USING ERRCODE = '42501';
			END IF;

			DELETE FROM documents.files
			WHERE tenant_id = p_tenant_id
			  AND deleted_at IS NOT NULL
			  AND file_deleted_at IS NOT NULL
			  AND file_deleted_at < p_cutoff;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			RETURN deleted_count;
		END;
		$$;

		REVOKE ALL ON FUNCTION documents.delete_removed_files(BIGINT, TIMESTAMPTZ) FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION documents.delete_removed_files(BIGINT, TIMESTAMPTZ) TO phoenix_tenant;

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
						'attendance_retention',
						'guardian_retention',
						'parent_message_retention',
						'parent_announcement_retention',
						'staff_notice_retention',
						'appointment_retention',
						'appointment_tombstone_retention',
						'grade_transition_retention',
						'removed_file_retention',
						'auth_event_retention',
						'data_access_log_retention',
						'deletion_log_retention',
						'delivery_retention',
						'file_cleanup_retention',
						'staff_cleanup_retention',
						'student_document_cleanup_retention',
						'care_withdrawal_retention',
						'submission_rate_limit_retention',
						'settled_enrollment_retention',
						'staff_sick_note_retention',
						'student_deletion_log_retention',
						'enrollment_deletion_log_retention',
						'enrollment_restoration_log_retention',
						'time_tracking_deletion_log_retention',
						'staff_master_data_change_retention',
						'personnel_number_change_retention',
						'guardian_financial_change_retention',
						'class_list_change_retention',
						'substitution_change_retention',
						'file_event_retention',
						'export_transfer_retention',
						'setting_audit_retention'
					)
				)
			);
`

// schoolContentRetentionDownSQL restores the subject check of 1.15.464 and
// removes the function.
const schoolContentRetentionDownSQL = `
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
						'attendance_retention',
						'guardian_retention',
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
						'settled_enrollment_retention',
						'staff_sick_note_retention',
						'student_deletion_log_retention',
						'enrollment_deletion_log_retention',
						'enrollment_restoration_log_retention',
						'time_tracking_deletion_log_retention',
						'staff_master_data_change_retention',
						'personnel_number_change_retention',
						'guardian_financial_change_retention',
						'class_list_change_retention',
						'substitution_change_retention',
						'file_event_retention',
						'export_transfer_retention',
						'setting_audit_retention'
					)
				)
			);

		DROP FUNCTION IF EXISTS documents.delete_removed_files(BIGINT, TIMESTAMPTZ);
`

func schoolContentRetentionUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackSchoolContentRetention(ctx, tx)
	if _, err := tx.ExecContext(ctx, schoolContentRetentionUpSQL); err != nil {
		return fmt.Errorf("prepare school content retention: %w", err)
	}
	return tx.Commit()
}

// schoolContentRetentionDown refuses while rows of its data kinds are logged:
// the restored check would reject them.
func schoolContentRetentionDown(ctx context.Context, db *bun.DB) error {
	var logged int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.data_deletions
		WHERE deletion_type IN (?) AND student_id IS NULL AND staff_id IS NULL`, bun.List(schoolContentRetentionKinds)).Scan(ctx, &logged); err != nil {
		return fmt.Errorf("check school content retention deletion log rows: %w", err)
	}
	if logged > 0 {
		return fmt.Errorf("cannot restore the data_deletions subject check while %d school content retention rows exist", logged)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackSchoolContentRetention(ctx, tx)
	if _, err := tx.ExecContext(ctx, schoolContentRetentionDownSQL); err != nil {
		return fmt.Errorf("revert school content retention: %w", err)
	}
	return tx.Commit()
}

func rollbackSchoolContentRetention(ctx context.Context, tx bun.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logRollbackFailure(ctx, err)
	}
}
