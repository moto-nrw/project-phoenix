package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	cleanupJobRetentionVersion     = "1.15.449"
	cleanupJobRetentionDescription = "Erledigte Aufräumaufträge, Betreuungsabmeldungen und Anmelde-Ratenbegrenzung nach fester Frist löschen (#4046)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     cleanupJobRetentionVersion,
		Description: cleanupJobRetentionDescription,
		DependsOn: []string{
			deliveryRetentionVersion,          // previous subject check of audit.data_deletions
			auditRetentionCapabilitiesVersion, // audit.authorize_retention_tenant
			parentDeclarationsVersion,         // auth.parent_declaration_permission_grants
		},
	})
	Migrations.MustRegister(cleanupJobRetentionUp, cleanupJobRetentionDown)
}

// cleanupJobRetentionKinds are the data kinds of the deletion log this
// migration adds.
var cleanupJobRetentionKinds = []string{
	"file_cleanup_retention",
	"staff_cleanup_retention",
	"student_document_cleanup_retention",
	"care_withdrawal_retention",
	"submission_rate_limit_retention",
}

// finishedCleanupFunctions are the functions of cleanupJobRetentionUpSQL that
// delete finished cleanup intents.
var finishedCleanupFunctions = []string{
	"documents.delete_finished_file_cleanups",
	"documents.delete_finished_announcement_attachment_cleanups",
	"users.delete_finished_staff_document_file_cleanups",
	"users.delete_finished_student_document_file_cleanups",
}

// cleanupJobRetentionUpSQL lets the nightly retention of a school delete its
// technical remains (#4046) and log each data kind as one row per run.
//
//   - The cleanup intents of stored files keep their direct DELETE revoked
//     from phoenix_tenant (1.15.257, 1.15.271, 1.15.332, 1.15.359): a request
//     must never drop an intent before its bytes are gone. One function per
//     table, in the pattern of 1.15.353 (#2850), runs as its owner, accepts
//     only the tenant of the calling transaction, deletes only finished
//     intents and refuses a cutoff newer than 30 days.
//   - users.staff_offboarding_cleanup, users.care_withdrawal_completions and
//     enrollment.submission_rate_limits already grant DELETE (1.15.374,
//     1.15.324, 1.15.066); RLS keeps the run to its school.
//   - The markers of 1.15.428 serve only its rollback. They go with their
//     relationship like the permission grants of the meta schema (1.15.387);
//     a marker of a relationship already gone can no longer change a
//     permission.
const cleanupJobRetentionUpSQL = `
		CREATE OR REPLACE FUNCTION documents.delete_finished_file_cleanups(
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
				RAISE EXCEPTION 'cleanup intent retention requires a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);
			IF NOT COALESCE(invoker_is_superuser, FALSE) AND p_cutoff > clock_timestamp() - INTERVAL '30 days' THEN
				RAISE EXCEPTION 'cleanup intent cutoff exceeds retention window' USING ERRCODE = '42501';
			END IF;

			DELETE FROM documents.file_cleanup
			WHERE tenant_id = p_tenant_id
			  AND cleaned_at IS NOT NULL
			  AND cleaned_at < p_cutoff;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			RETURN deleted_count;
		END;
		$$;

		CREATE OR REPLACE FUNCTION documents.delete_finished_announcement_attachment_cleanups(
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
				RAISE EXCEPTION 'cleanup intent retention requires a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);
			IF NOT COALESCE(invoker_is_superuser, FALSE) AND p_cutoff > clock_timestamp() - INTERVAL '30 days' THEN
				RAISE EXCEPTION 'cleanup intent cutoff exceeds retention window' USING ERRCODE = '42501';
			END IF;

			DELETE FROM documents.announcement_attachment_cleanup
			WHERE tenant_id = p_tenant_id
			  AND cleaned_at IS NOT NULL
			  AND cleaned_at < p_cutoff;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			RETURN deleted_count;
		END;
		$$;

		CREATE OR REPLACE FUNCTION users.delete_finished_staff_document_file_cleanups(
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
				RAISE EXCEPTION 'cleanup intent retention requires a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);
			IF NOT COALESCE(invoker_is_superuser, FALSE) AND p_cutoff > clock_timestamp() - INTERVAL '30 days' THEN
				RAISE EXCEPTION 'cleanup intent cutoff exceeds retention window' USING ERRCODE = '42501';
			END IF;

			DELETE FROM users.staff_document_file_cleanup
			WHERE tenant_id = p_tenant_id
			  AND cleaned_at IS NOT NULL
			  AND cleaned_at < p_cutoff;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			RETURN deleted_count;
		END;
		$$;

		CREATE OR REPLACE FUNCTION users.delete_finished_student_document_file_cleanups(
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
				RAISE EXCEPTION 'cleanup intent retention requires a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);
			IF NOT COALESCE(invoker_is_superuser, FALSE) AND p_cutoff > clock_timestamp() - INTERVAL '30 days' THEN
				RAISE EXCEPTION 'cleanup intent cutoff exceeds retention window' USING ERRCODE = '42501';
			END IF;

			DELETE FROM users.student_document_file_cleanup
			WHERE tenant_id = p_tenant_id
			  AND cleaned_at IS NOT NULL
			  AND cleaned_at < p_cutoff;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			RETURN deleted_count;
		END;
		$$;

		REVOKE ALL ON FUNCTION documents.delete_finished_file_cleanups(BIGINT, TIMESTAMPTZ) FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION documents.delete_finished_file_cleanups(BIGINT, TIMESTAMPTZ) TO phoenix_tenant;
		REVOKE ALL ON FUNCTION documents.delete_finished_announcement_attachment_cleanups(BIGINT, TIMESTAMPTZ) FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION documents.delete_finished_announcement_attachment_cleanups(BIGINT, TIMESTAMPTZ) TO phoenix_tenant;
		REVOKE ALL ON FUNCTION users.delete_finished_staff_document_file_cleanups(BIGINT, TIMESTAMPTZ) FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION users.delete_finished_staff_document_file_cleanups(BIGINT, TIMESTAMPTZ) TO phoenix_tenant;
		REVOKE ALL ON FUNCTION users.delete_finished_student_document_file_cleanups(BIGINT, TIMESTAMPTZ) FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION users.delete_finished_student_document_file_cleanups(BIGINT, TIMESTAMPTZ) TO phoenix_tenant;

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

		DELETE FROM auth.parent_declaration_permission_grants AS marker
		WHERE NOT EXISTS (
			SELECT 1 FROM users.student_guardian_relationships AS relationship
			WHERE relationship.tenant_id = marker.tenant_id AND relationship.id = marker.relationship_id
		);
		ALTER TABLE auth.parent_declaration_permission_grants
			ADD CONSTRAINT fk_parent_declaration_permission_grant_relationship
			FOREIGN KEY (tenant_id, relationship_id)
			REFERENCES users.student_guardian_relationships(tenant_id, id) ON DELETE CASCADE;
`

const cleanupJobRetentionDownSQL = `
		ALTER TABLE auth.parent_declaration_permission_grants
			DROP CONSTRAINT IF EXISTS fk_parent_declaration_permission_grant_relationship;

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
						'delivery_retention'
					)
				)
			);

		DROP FUNCTION IF EXISTS documents.delete_finished_file_cleanups(BIGINT, TIMESTAMPTZ);
		DROP FUNCTION IF EXISTS documents.delete_finished_announcement_attachment_cleanups(BIGINT, TIMESTAMPTZ);
		DROP FUNCTION IF EXISTS users.delete_finished_staff_document_file_cleanups(BIGINT, TIMESTAMPTZ);
		DROP FUNCTION IF EXISTS users.delete_finished_student_document_file_cleanups(BIGINT, TIMESTAMPTZ);
`

func cleanupJobRetentionUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()
	if _, err := tx.ExecContext(ctx, cleanupJobRetentionUpSQL); err != nil {
		return fmt.Errorf("allow cleanup job retention: %w", err)
	}
	return tx.Commit()
}

func cleanupJobRetentionDown(ctx context.Context, db *bun.DB) error {
	var logged int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.data_deletions
		WHERE deletion_type IN (?) AND student_id IS NULL AND staff_id IS NULL`, bun.List(cleanupJobRetentionKinds)).Scan(ctx, &logged); err != nil {
		return fmt.Errorf("check cleanup job retention deletion log rows: %w", err)
	}
	if logged > 0 {
		return fmt.Errorf("cannot restore the data_deletions subject check while %d cleanup job retention rows exist", logged)
	}
	if _, err := db.ExecContext(ctx, cleanupJobRetentionDownSQL); err != nil {
		return fmt.Errorf("revoke cleanup job retention: %w", err)
	}
	return nil
}
