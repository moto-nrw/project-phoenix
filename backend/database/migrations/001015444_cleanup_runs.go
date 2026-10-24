package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	cleanupRunsVersion     = "1.15.444"
	cleanupRunsDescription = "Nächtliche Löschung: letzten erfolgreichen Lauf je Schule speichern und Löschprotokoll ohne Personenbezug erlauben (#3950)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     cleanupRunsVersion,
		Description: cleanupRunsDescription,
		DependsOn:   []string{studentDepartureExceptionsVersion, preserveStudentDeletionAuditVersion},
	})

	Migrations.MustRegister(cleanupRunsUp, cleanupRunsDown)
}

// cleanupRunsUp stores the last successful nightly run of each cleanup job per
// school, so a run the process missed is caught up after a restart (#3950).
// The Worker writes the row in the tenant transaction of the run itself: it
// exists only when the deletions committed.
//
// It also lets a retention run log a whole data kind in audit.data_deletions.
// Until now every row named exactly one child or one staff member. A
// retention step that deletes rows of many people, or rows without a person
// such as unregistered tag scans, records one row per data kind with its
// count instead. Only these retention types may leave both subjects empty.
func cleanupRunsUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS platform.cleanup_runs (
			tenant_id    BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			job_id       TEXT NOT NULL CHECK (job_id <> ''),
			succeeded_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (tenant_id, job_id)
		);
		COMMENT ON TABLE platform.cleanup_runs IS
			'Letzter erfolgreicher Lauf je nächtlichem Löschjob und Schule (#3950). Wurde ein geplanter Lauf verpasst, holt der Worker ihn nach.';

		GRANT SELECT, INSERT, UPDATE ON platform.cleanup_runs TO phoenix_tenant;

		ALTER TABLE audit.data_deletions
			DROP CONSTRAINT IF EXISTS chk_data_deletions_subject_xor;
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
						'deletion_log_retention'
					)
				)
			);
	`); err != nil {
		return fmt.Errorf("create cleanup runs: %w", err)
	}
	if err := provisionTenantRLS(ctx, tx, "platform.cleanup_runs"); err != nil {
		return err
	}
	return tx.Commit()
}

func cleanupRunsDown(ctx context.Context, db *bun.DB) error {
	var aggregateRows int
	if err := db.NewRaw(`
		SELECT COUNT(*) FROM audit.data_deletions
		WHERE student_id IS NULL AND staff_id IS NULL
	`).Scan(ctx, &aggregateRows); err != nil {
		return fmt.Errorf("check retention deletion log rows: %w", err)
	}
	if aggregateRows > 0 {
		return fmt.Errorf("cannot restore the data_deletions subject check while %d retention rows without a subject exist", aggregateRows)
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE audit.data_deletions
			DROP CONSTRAINT IF EXISTS chk_data_deletions_subject;
		ALTER TABLE audit.data_deletions
			ADD CONSTRAINT chk_data_deletions_subject_xor CHECK ((student_id IS NOT NULL) <> (staff_id IS NOT NULL));
		DROP TABLE IF EXISTS platform.cleanup_runs;
	`); err != nil {
		return fmt.Errorf("drop cleanup runs: %w", err)
	}
	return nil
}
