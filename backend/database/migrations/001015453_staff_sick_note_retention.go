package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffSickNoteRetentionVersion     = "1.15.453"
	staffSickNoteRetentionDescription = "AU-Bescheinigungen nach der Löschfrist der Schule samt Datei löschen (#3961)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffSickNoteRetentionVersion,
		Description: staffSickNoteRetentionDescription,
		DependsOn: []string{
			settledEnrollmentRetentionVersion, // previous subject check of audit.data_deletions
			auditRetentionCapabilitiesVersion, // audit.authorize_retention_tenant
			staffDocumentsVersion,             // users.staff_documents and its file cleanup queue
		},
	})
	Migrations.MustRegister(staffSickNoteRetentionUp, staffSickNoteRetentionDown)
}

// staffSickNoteRetentionKind is the data kind of the deletion log this
// migration adds.
const staffSickNoteRetentionKind = "staff_sick_note_retention"

// staffSickNoteRetentionUpSQL lets the nightly retention of a school delete
// its expired sick notes (AU-Bescheinigungen, #3961).
//
// phoenix_tenant keeps no DELETE on users.staff_documents (1.15.257): a
// request only soft-deletes, so the metadata row stays the trail of a
// document. The function runs as its owner in the pattern of 1.15.449,
// accepts only the tenant of the calling transaction, deletes only the
// au_bescheinigung category and refuses a cutoff outside the 90- to 1095-day
// frame of the school. Each deleted row whose file still exists queues its
// stored name in users.staff_document_file_cleanup, the queue the staff
// document file sweep already drains; an intent finished by the upload is
// queued again.
const staffSickNoteRetentionUpSQL = `
		CREATE OR REPLACE FUNCTION users.delete_expired_staff_sick_notes(
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
			earliest_allowed_cutoff TIMESTAMPTZ;
			latest_allowed_cutoff TIMESTAMPTZ;
		BEGIN
			IF p_cutoff IS NULL THEN
				RAISE EXCEPTION 'sick note retention requires a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);
			latest_allowed_cutoff := (
				((clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date - 90)::timestamp
			) AT TIME ZONE 'Europe/Berlin';
			earliest_allowed_cutoff := (
				((clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date - 1095)::timestamp
			) AT TIME ZONE 'Europe/Berlin';
			IF NOT COALESCE(invoker_is_superuser, FALSE)
				AND (p_cutoff > latest_allowed_cutoff OR p_cutoff < earliest_allowed_cutoff) THEN
				RAISE EXCEPTION 'sick note cutoff exceeds retention window' USING ERRCODE = '42501';
			END IF;

			WITH removed AS (
				DELETE FROM users.staff_documents
				WHERE tenant_id = p_tenant_id
				  AND category = 'au_bescheinigung'
				  AND created_at < p_cutoff
				RETURNING tenant_id, staff_id, filename_stored, file_deleted_at
			), queued AS (
				INSERT INTO users.staff_document_file_cleanup (tenant_id, staff_id, filename_stored, retry_after)
				SELECT tenant_id, staff_id, filename_stored, clock_timestamp()
				FROM removed
				WHERE file_deleted_at IS NULL
				ON CONFLICT (tenant_id, filename_stored)
				DO UPDATE SET cleaned_at = NULL, retry_after = EXCLUDED.retry_after
			)
			SELECT COUNT(*) INTO deleted_count FROM removed;
			RETURN deleted_count;
		END;
		$$;

		REVOKE ALL ON FUNCTION users.delete_expired_staff_sick_notes(BIGINT, TIMESTAMPTZ) FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION users.delete_expired_staff_sick_notes(BIGINT, TIMESTAMPTZ) TO phoenix_tenant;

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
						'settled_enrollment_retention',
						'staff_sick_note_retention'
					)
				)
			);
`

// staffSickNoteRetentionDownSQL restores the subject check of 1.15.452 and
// drops the function.
const staffSickNoteRetentionDownSQL = `
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

		DROP FUNCTION IF EXISTS users.delete_expired_staff_sick_notes(BIGINT, TIMESTAMPTZ);
`

func staffSickNoteRetentionUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()
	if _, err := tx.ExecContext(ctx, staffSickNoteRetentionUpSQL); err != nil {
		return fmt.Errorf("allow staff sick note retention: %w", err)
	}
	return tx.Commit()
}

func staffSickNoteRetentionDown(ctx context.Context, db *bun.DB) error {
	var logged int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.data_deletions
		WHERE deletion_type = ? AND student_id IS NULL AND staff_id IS NULL`, staffSickNoteRetentionKind).Scan(ctx, &logged); err != nil {
		return fmt.Errorf("check staff sick note retention deletion log rows: %w", err)
	}
	if logged > 0 {
		return fmt.Errorf("cannot restore the data_deletions subject check while %d staff sick note retention rows exist", logged)
	}
	if _, err := db.ExecContext(ctx, staffSickNoteRetentionDownSQL); err != nil {
		return fmt.Errorf("revoke staff sick note retention: %w", err)
	}
	return nil
}
