package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const cleanupRunFailuresVersion = "1.15.832"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     cleanupRunFailuresVersion,
		Description: "Nächtliche Löschung: Fehlschläge in Folge je Schule und Job speichern (#6327)",
		DependsOn: []string{
			parentBillingSEPAMandateImportsVersion, // the last migration before this one
			cleanupRunsVersion,                     // the table this one extends
		},
	})
	Migrations.MustRegister(cleanupRunFailuresUp, cleanupRunFailuresDown)
}

// cleanupRunFailuresUpSQL keeps the failure streak of a nightly cleanup job
// next to its last success (#6327). The Worker spaces the retries of a
// failing job by this streak and raises an alert once it is long enough; a
// success sets it back to zero. A job that never succeeded has a row with
// its failures only, so succeeded_at becomes nullable.
const cleanupRunFailuresUpSQL = `
ALTER TABLE platform.cleanup_runs
	ALTER COLUMN succeeded_at DROP NOT NULL,
	ADD COLUMN IF NOT EXISTS consecutive_failures INTEGER NOT NULL DEFAULT 0,
	ADD COLUMN IF NOT EXISTS last_failed_at TIMESTAMPTZ;
ALTER TABLE platform.cleanup_runs
	DROP CONSTRAINT IF EXISTS chk_cleanup_runs_state;
ALTER TABLE platform.cleanup_runs
	ADD CONSTRAINT chk_cleanup_runs_state CHECK (
		consecutive_failures >= 0
		AND (consecutive_failures = 0) = (last_failed_at IS NULL)
		AND (succeeded_at IS NOT NULL OR consecutive_failures > 0)
	);
COMMENT ON TABLE platform.cleanup_runs IS
	'Letzter erfolgreicher Lauf und Fehlschläge in Folge je nächtlichem Löschjob und Schule (#3950, #6327). Wurde ein geplanter Lauf verpasst, holt der Worker ihn nach; nach Fehlschlägen wartet er länger.';
COMMENT ON COLUMN platform.cleanup_runs.consecutive_failures IS
	'Fehlgeschlagene Läufe seit dem letzten Erfolg (#6327); ein Erfolg setzt sie auf 0.';
COMMENT ON COLUMN platform.cleanup_runs.last_failed_at IS
	'Letzter fehlgeschlagener Lauf seit dem letzten Erfolg (#6327); NULL ohne Fehlschlag.';
`

// cleanupRunFailuresDownSQL drops the streak. Rows of jobs that never
// succeeded hold nothing else and go with it.
const cleanupRunFailuresDownSQL = `
DELETE FROM platform.cleanup_runs WHERE succeeded_at IS NULL;
ALTER TABLE platform.cleanup_runs
	DROP CONSTRAINT IF EXISTS chk_cleanup_runs_state,
	DROP COLUMN IF EXISTS last_failed_at,
	DROP COLUMN IF EXISTS consecutive_failures,
	ALTER COLUMN succeeded_at SET NOT NULL;
`

func cleanupRunFailuresUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, cleanupRunFailuresUpSQL); err != nil {
		return fmt.Errorf("add the failure streak of cleanup runs: %w", err)
	}
	return nil
}

func cleanupRunFailuresDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, cleanupRunFailuresDownSQL); err != nil {
		return fmt.Errorf("drop the failure streak of cleanup runs: %w", err)
	}
	return nil
}
