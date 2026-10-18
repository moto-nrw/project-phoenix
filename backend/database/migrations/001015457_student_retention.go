package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const studentRetentionVersion = "1.15.457"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentRetentionVersion,
		Description: "Let the nightly run record the automatic deletion of a child without an actor (#3962)",
		DependsOn: []string{
			studentCareExitsVersion,       // the retention_expired reason of audit.student_deletions
			demoVisitorErasureVersion,     // the last migration before this one
			staffSickNoteRetentionVersion, // update its file cleanup queue for the worker clock
		},
	})
	Migrations.MustRegister(studentRetentionUp, studentRetentionDown)
}

// studentRetentionUpSQL lets a student tombstone name no actor (#3962). The
// nightly run deletes a child once the school's retention window after the
// last care day has passed; nobody confirmed that deletion, so the tombstone
// records no account. Only the reason retention_expired may go without one:
// every deletion a person confirms keeps its actor.
const studentRetentionUpSQL = `
		ALTER TABLE audit.student_deletions
			ALTER COLUMN actor_account_id DROP NOT NULL;
		ALTER TABLE audit.student_deletions
			DROP CONSTRAINT IF EXISTS chk_student_deletions_actor;
		ALTER TABLE audit.student_deletions
			ADD CONSTRAINT chk_student_deletions_actor
			CHECK (actor_account_id IS NOT NULL OR reason = 'retention_expired');
`

const studentRetentionDownSQL = `
		ALTER TABLE audit.student_deletions
			DROP CONSTRAINT IF EXISTS chk_student_deletions_actor;
		ALTER TABLE audit.student_deletions
			ALTER COLUMN actor_account_id SET NOT NULL;
`

// studentRetentionSickNoteCleanupFunctionPrefix and its suffix update the
// function introduced in 1.15.453. Its queue must be ready when the same
// cleanup run immediately reaches the file sweep, even if the application
// clock slightly lags PostgreSQL. The retry expression is fixed by each
// migration direction.
const studentRetentionSickNoteCleanupFunctionPrefix = `
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
			SELECT tenant_id, staff_id, filename_stored, `

const studentRetentionSickNoteCleanupFunctionSuffix = `
			FROM removed
			WHERE file_deleted_at IS NULL
			ON CONFLICT (tenant_id, filename_stored)
			DO UPDATE SET cleaned_at = NULL, retry_after = EXCLUDED.retry_after
		)
		SELECT COUNT(*) INTO deleted_count FROM removed;
		RETURN deleted_count;
	END;
	$$;
`

const studentRetentionSickNoteCleanupImmediateSQL = studentRetentionSickNoteCleanupFunctionPrefix + `TIMESTAMPTZ 'epoch'` + studentRetentionSickNoteCleanupFunctionSuffix

const studentRetentionSickNoteCleanupDeferredSQL = studentRetentionSickNoteCleanupFunctionPrefix + `clock_timestamp()` + studentRetentionSickNoteCleanupFunctionSuffix

func studentRetentionUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, studentRetentionUpSQL); err != nil {
		return fmt.Errorf("allow automatic student deletions: %w", err)
	}
	if _, err := db.ExecContext(ctx, studentRetentionSickNoteCleanupImmediateSQL); err != nil {
		return fmt.Errorf("make sick note file cleanup immediately eligible: %w", err)
	}
	return nil
}

// studentRetentionDown refuses while automatic deletions are recorded: their
// tombstones have no actor to restore.
func studentRetentionDown(ctx context.Context, db *bun.DB) error {
	var automatic int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.student_deletions WHERE actor_account_id IS NULL`).Scan(ctx, &automatic); err != nil {
		return fmt.Errorf("check automatic student deletions: %w", err)
	}
	if automatic > 0 {
		return fmt.Errorf("cannot require an actor on audit.student_deletions while %d automatic deletions exist", automatic)
	}
	if _, err := db.ExecContext(ctx, studentRetentionSickNoteCleanupDeferredSQL); err != nil {
		return fmt.Errorf("restore sick note file cleanup retry time: %w", err)
	}
	if _, err := db.ExecContext(ctx, studentRetentionDownSQL); err != nil {
		return fmt.Errorf("require an actor on student deletions: %w", err)
	}
	return nil
}
