package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	protocolRetentionVersion     = "1.15.454"
	protocolRetentionDescription = "Feste Löschfristen für fachliche Protokolle und das Einstellungsprotokoll (#4044)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     protocolRetentionVersion,
		Description: protocolRetentionDescription,
		DependsOn: []string{
			staffSickNoteRetentionVersion,     // previous subject check of audit.data_deletions
			auditRetentionCapabilitiesVersion, // audit.authorize_retention_tenant
		},
	})
	Migrations.MustRegister(protocolRetentionUp, protocolRetentionDown)
}

// protocolRetentionKinds are the data kinds of the deletion log this
// migration adds: one per protocol of #4044.
var protocolRetentionKinds = []string{
	"student_deletion_log_retention",
	"enrollment_deletion_log_retention",
	"enrollment_restoration_log_retention",
	"time_tracking_deletion_log_retention",
	"staff_master_data_change_retention",
	"personnel_number_change_retention",
	"guardian_financial_change_retention",
	"class_list_change_retention",
	"substitution_change_retention",
	"file_event_retention",
	"export_transfer_retention",
	"setting_audit_retention",
}

// protocolRetentionUpSQL gives the protocols of #4044 their fixed retention
// (ADR 0048: protocols have fixed windows for every school).
//
//   - Three years after the event, like the deletion log, for the protocols
//     that prove a deletion or a change: deletions of children and
//     enrollments, restored enrollments, deleted time-tracking entries and
//     the changes of staff master data, personnel numbers and guardians'
//     payment data.
//   - Until the end of the year after the event, like the access log, for
//     the protocols of daily operation: class-list matches, substitutions
//     and file events of the Audit Platform, and the export journal of the
//     Export Transfer owner.
//
// The audit tables stay append-only for phoenix_tenant (1.15.225 and the
// migrations creating them). Their functions follow 1.15.447: they run as
// their owner, accept only the tenant of the calling transaction and refuse
// a cutoff newer than the window, so a tenant role can never delete young or
// foreign rows. The Audit Platform's ten protocols share one function that
// names each table; the export journal has its own.
//
// config.setting_audit (three years) already grants DELETE to the tenant
// role and keeps it; RLS keeps the run to its school. Its actor now goes
// SET NULL with the account, like the enrollment deletion protocols, so a
// setting change no longer blocks deleting the account that made it.

const protocolRetentionUpSQL = `
		CREATE OR REPLACE FUNCTION audit.delete_expired_protocol(
			p_tenant_id BIGINT,
			p_protocol TEXT,
			p_cutoff TIMESTAMPTZ
		)
		RETURNS BIGINT
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog, audit
		AS $$
		DECLARE
			latest_allowed_cutoff TIMESTAMPTZ;
			deleted_count BIGINT;
			invoker_is_superuser BOOLEAN;
		BEGIN
			IF p_cutoff IS NULL THEN
				RAISE EXCEPTION 'protocol retention requires a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);

			CASE p_protocol
				WHEN 'student_deletion_log_retention',
					'enrollment_deletion_log_retention',
					'enrollment_restoration_log_retention',
					'time_tracking_deletion_log_retention',
					'staff_master_data_change_retention',
					'personnel_number_change_retention',
					'guardian_financial_change_retention' THEN
					latest_allowed_cutoff := clock_timestamp() - INTERVAL '3 years';
				WHEN 'class_list_change_retention',
					'substitution_change_retention',
					'file_event_retention' THEN
					latest_allowed_cutoff := (
						date_trunc('year', clock_timestamp() AT TIME ZONE 'Europe/Berlin') - INTERVAL '1 year'
					) AT TIME ZONE 'Europe/Berlin';
				ELSE
					RAISE EXCEPTION 'unknown protocol %', p_protocol USING ERRCODE = '22023';
			END CASE;
			IF NOT COALESCE(invoker_is_superuser, FALSE) AND p_cutoff > latest_allowed_cutoff THEN
				RAISE EXCEPTION 'protocol cutoff exceeds retention window' USING ERRCODE = '42501';
			END IF;

			CASE p_protocol
				WHEN 'student_deletion_log_retention' THEN
					DELETE FROM audit.student_deletions WHERE tenant_id = p_tenant_id AND deleted_at < p_cutoff;
				WHEN 'enrollment_deletion_log_retention' THEN
					DELETE FROM audit.enrollment_deletions WHERE tenant_id = p_tenant_id AND deleted_at < p_cutoff;
				WHEN 'enrollment_restoration_log_retention' THEN
					DELETE FROM audit.enrollment_restorations WHERE tenant_id = p_tenant_id AND restored_at < p_cutoff;
				WHEN 'time_tracking_deletion_log_retention' THEN
					DELETE FROM audit.time_tracking_deletions WHERE tenant_id = p_tenant_id AND occurred_at < p_cutoff;
				WHEN 'staff_master_data_change_retention' THEN
					DELETE FROM audit.staff_master_data_changes WHERE tenant_id = p_tenant_id AND occurred_at < p_cutoff;
				WHEN 'personnel_number_change_retention' THEN
					DELETE FROM audit.personnel_number_changes WHERE tenant_id = p_tenant_id AND occurred_at < p_cutoff;
				WHEN 'guardian_financial_change_retention' THEN
					DELETE FROM audit.guardian_financial_changes WHERE tenant_id = p_tenant_id AND occurred_at < p_cutoff;
				WHEN 'class_list_change_retention' THEN
					DELETE FROM audit.class_list_entry_changes WHERE tenant_id = p_tenant_id AND occurred_at < p_cutoff;
				WHEN 'substitution_change_retention' THEN
					DELETE FROM audit.substitution_changes WHERE tenant_id = p_tenant_id AND created_at < p_cutoff;
				WHEN 'file_event_retention' THEN
					DELETE FROM audit.file_events WHERE tenant_id = p_tenant_id AND created_at < p_cutoff;
			END CASE;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			RAISE LOG 'audit retention executed: tenant_id=%, capability=%, cutoff=%, rows_deleted=%',
				p_tenant_id, p_protocol, p_cutoff, deleted_count;
			RETURN deleted_count;
		END;
		$$;

		CREATE OR REPLACE FUNCTION audit.delete_expired_export_transfers(
			p_tenant_id BIGINT,
			p_cutoff TIMESTAMPTZ
		)
		RETURNS BIGINT
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog, audit
		AS $$
		DECLARE
			latest_allowed_cutoff TIMESTAMPTZ;
			deleted_count BIGINT;
			invoker_is_superuser BOOLEAN;
		BEGIN
			IF p_cutoff IS NULL THEN
				RAISE EXCEPTION 'export transfer retention requires a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);

			latest_allowed_cutoff := (
				date_trunc('year', clock_timestamp() AT TIME ZONE 'Europe/Berlin') - INTERVAL '1 year'
			) AT TIME ZONE 'Europe/Berlin';
			IF NOT COALESCE(invoker_is_superuser, FALSE) AND p_cutoff > latest_allowed_cutoff THEN
				RAISE EXCEPTION 'export transfer cutoff exceeds retention window' USING ERRCODE = '42501';
			END IF;

			DELETE FROM audit.export_transfers
			WHERE tenant_id = p_tenant_id
			  AND created_at < p_cutoff;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			RAISE LOG 'audit retention executed: tenant_id=%, capability=export_transfers, cutoff=%, rows_deleted=%',
				p_tenant_id, p_cutoff, deleted_count;
			RETURN deleted_count;
		END;
		$$;

		REVOKE ALL ON FUNCTION audit.delete_expired_protocol(BIGINT, TEXT, TIMESTAMPTZ) FROM PUBLIC;
		REVOKE ALL ON FUNCTION audit.delete_expired_export_transfers(BIGINT, TIMESTAMPTZ) FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION audit.delete_expired_protocol(BIGINT, TEXT, TIMESTAMPTZ) TO phoenix_tenant;
		GRANT EXECUTE ON FUNCTION audit.delete_expired_export_transfers(BIGINT, TIMESTAMPTZ) TO phoenix_tenant;

		ALTER TABLE config.setting_audit
			DROP CONSTRAINT IF EXISTS setting_audit_changed_by_fkey;
		ALTER TABLE config.setting_audit
			ADD CONSTRAINT setting_audit_changed_by_fkey
			FOREIGN KEY (changed_by) REFERENCES auth.accounts(id) ON DELETE SET NULL;

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

// protocolRetentionDownSQL restores the subject check of 1.15.453 and the
// actor key of 1.15.25, and drops the functions.
const protocolRetentionDownSQL = `
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

		ALTER TABLE config.setting_audit
			DROP CONSTRAINT IF EXISTS setting_audit_changed_by_fkey;
		ALTER TABLE config.setting_audit
			ADD CONSTRAINT setting_audit_changed_by_fkey
			FOREIGN KEY (changed_by) REFERENCES auth.accounts(id);

		DROP FUNCTION IF EXISTS audit.delete_expired_protocol(BIGINT, TEXT, TIMESTAMPTZ);
		DROP FUNCTION IF EXISTS audit.delete_expired_export_transfers(BIGINT, TIMESTAMPTZ);
`

func protocolRetentionUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()
	if _, err := tx.ExecContext(ctx, protocolRetentionUpSQL); err != nil {
		return fmt.Errorf("create protocol retention capabilities: %w", err)
	}
	return tx.Commit()
}

func protocolRetentionDown(ctx context.Context, db *bun.DB) error {
	var logged int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.data_deletions
		WHERE deletion_type IN (?) AND student_id IS NULL AND staff_id IS NULL`, bun.List(protocolRetentionKinds)).Scan(ctx, &logged); err != nil {
		return fmt.Errorf("check protocol retention deletion log rows: %w", err)
	}
	if logged > 0 {
		return fmt.Errorf("cannot restore the data_deletions subject check while %d protocol retention rows exist", logged)
	}
	if _, err := db.ExecContext(ctx, protocolRetentionDownSQL); err != nil {
		return fmt.Errorf("drop protocol retention capabilities: %w", err)
	}
	return nil
}
