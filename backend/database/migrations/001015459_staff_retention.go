package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const staffRetentionVersion = "1.15.459"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffRetentionVersion,
		Description: "Ausgetretenes Personal nach der Zeiterfassungsfrist der Schule löschen (#3963)",
		DependsOn: []string{
			studentRetentionVersion,           // the last migration before this one
			staffSickNoteRetentionVersion,     // the file cleanup queue of staff documents
			auditRetentionCapabilitiesVersion, // audit.authorize_retention_tenant
			staffOwnerCutoverVersion,          // users.staff_school_memberships and users.staff_employment_profiles
		},
	})
	Migrations.MustRegister(staffRetentionUp, staffRetentionDown)
}

// staffRetentionUpSQL lets the nightly run of a school erase a staff member
// who left more than the school's time-tracking window ago (#3963).
//
// The membership stays as a tombstone: records of other people (check-ins,
// approvals, notes, appointments) still name it as their actor, and their
// foreign keys do not cascade. personal_data_erased_at marks the tombstone
// once its staff records are gone, so the nightly run erases and logs each
// staff member once.
//
// phoenix_tenant keeps no DELETE on users.staff_documents and
// users.staff_qualifications (1.15.257): a request only soft-deletes them.
// users.erase_retired_staff_records runs as its owner in the pattern of
// 1.15.453, accepts only the tenant of the calling transaction, refuses a
// staff member who has not left before p_retired_before and a cutoff outside
// the 730- to 2920-day frame of the time-tracking window. The frame allows
// one more day at both ends: the nightly run computes the cutoff from its
// own Berlin day, which can differ from the database's day around midnight.
// It deletes every
// Workforce record of the staff member and queues the stored names of
// document files that still exist in users.staff_document_file_cleanup.
const staffRetentionUpSQL = `
		ALTER TABLE users.staff_school_memberships
			ADD COLUMN IF NOT EXISTS personal_data_erased_at TIMESTAMPTZ;
		ALTER TABLE users.staff_school_memberships
			DROP CONSTRAINT IF EXISTS chk_staff_school_memberships_erased_retired;
		ALTER TABLE users.staff_school_memberships
			ADD CONSTRAINT chk_staff_school_memberships_erased_retired
			CHECK (personal_data_erased_at IS NULL OR deleted_at IS NOT NULL);

		CREATE OR REPLACE FUNCTION users.erase_retired_staff_records(
			p_tenant_id BIGINT,
			p_staff_id BIGINT,
			p_retired_before TIMESTAMPTZ
		)
		RETURNS JSONB
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog
		AS $$
		DECLARE
			counts JSONB := '{}'::jsonb;
			deleted_count BIGINT;
			invoker_is_superuser BOOLEAN;
			earliest_allowed_cutoff TIMESTAMPTZ;
			latest_allowed_cutoff TIMESTAMPTZ;
		BEGIN
			IF p_staff_id IS NULL OR p_staff_id <= 0 OR p_retired_before IS NULL THEN
				RAISE EXCEPTION 'staff retention requires a staff member and a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);
			latest_allowed_cutoff := (
				((clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date - 729)::timestamp
			) AT TIME ZONE 'Europe/Berlin';
			earliest_allowed_cutoff := (
				((clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date - 2921)::timestamp
			) AT TIME ZONE 'Europe/Berlin';
			IF NOT COALESCE(invoker_is_superuser, FALSE)
				AND (p_retired_before > latest_allowed_cutoff OR p_retired_before < earliest_allowed_cutoff) THEN
				RAISE EXCEPTION 'staff retention cutoff exceeds the time-tracking window' USING ERRCODE = '42501';
			END IF;
			PERFORM 1 FROM users.staff_school_memberships
			WHERE tenant_id = p_tenant_id AND id = p_staff_id
			  AND deleted_at IS NOT NULL AND deleted_at < p_retired_before;
			IF NOT FOUND THEN
				RAISE EXCEPTION 'staff member has not left before the cutoff' USING ERRCODE = '42501';
			END IF;

			DELETE FROM education.group_substitution
			WHERE tenant_id = p_tenant_id AND (regular_staff_id = p_staff_id OR substitute_staff_id = p_staff_id);
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('group_substitutions', deleted_count);

			DELETE FROM schedule.staff_shift_series WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('staff_shift_series', deleted_count);
			DELETE FROM schedule.staff_shifts WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('staff_shifts', deleted_count);

			DELETE FROM active.work_sessions WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('work_sessions', deleted_count);
			DELETE FROM active.staff_absences WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('staff_absences', deleted_count);

			DELETE FROM active.staff_balance_adjustments WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('balance_adjustments', deleted_count);
			DELETE FROM active.staff_month_balance_snapshots WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('month_balance_snapshots', deleted_count);
			DELETE FROM active.staff_vacation_openings WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('vacation_openings', deleted_count);
			DELETE FROM active.staff_vacation_quota_changes WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('vacation_quota_changes', deleted_count);
			DELETE FROM active.staff_vacation_quota WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('vacation_quotas', deleted_count);
			DELETE FROM active.staff_absence_type_allowance_changes WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('absence_allowance_changes', deleted_count);
			DELETE FROM active.staff_absence_type_allowances WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('absence_allowances', deleted_count);

			DELETE FROM config.staff_work_schedules WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('work_schedules', deleted_count);
			DELETE FROM config.staff_target_overrides WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('target_overrides', deleted_count);

			DELETE FROM users.staff_master_data WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('master_data', deleted_count);
			DELETE FROM users.staff_financial_data WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('financial_data', deleted_count);
			DELETE FROM users.staff_qualifications WHERE tenant_id = p_tenant_id AND staff_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('qualifications', deleted_count);
			WITH removed AS (
				DELETE FROM users.staff_documents
				WHERE tenant_id = p_tenant_id
				  AND staff_id = p_staff_id
				  AND category <> 'au_bescheinigung'
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
			counts := counts || jsonb_build_object('documents', deleted_count);
			DELETE FROM users.staff_employment_profiles WHERE tenant_id = p_tenant_id AND membership_id = p_staff_id;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			counts := counts || jsonb_build_object('employment_profiles', deleted_count);

			RETURN counts;
		END;
		$$;

		REVOKE ALL ON FUNCTION users.erase_retired_staff_records(BIGINT, BIGINT, TIMESTAMPTZ) FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION users.erase_retired_staff_records(BIGINT, BIGINT, TIMESTAMPTZ) TO phoenix_tenant;
`

// staffRetentionDownSQL drops the function and the erasure marker. An erased
// tombstone then looks like any retired staff member, so a later run erases
// its (already empty) records again and logs it a second time.
const staffRetentionDownSQL = `
		DROP FUNCTION IF EXISTS users.erase_retired_staff_records(BIGINT, BIGINT, TIMESTAMPTZ);
		ALTER TABLE users.staff_school_memberships
			DROP CONSTRAINT IF EXISTS chk_staff_school_memberships_erased_retired;
		ALTER TABLE users.staff_school_memberships
			DROP COLUMN IF EXISTS personal_data_erased_at;
`

func staffRetentionUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()
	if _, err := tx.ExecContext(ctx, staffRetentionUpSQL); err != nil {
		return fmt.Errorf("allow staff retention: %w", err)
	}
	return tx.Commit()
}

func staffRetentionDown(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()
	if _, err := tx.ExecContext(ctx, staffRetentionDownSQL); err != nil {
		return fmt.Errorf("revoke staff retention: %w", err)
	}
	return tx.Commit()
}
