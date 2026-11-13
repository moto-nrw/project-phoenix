package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const statusDayProvenanceVersion = "20261113102344"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     statusDayProvenanceVersion,
		Description: "Status-Tage behalten Quelle und Meldezeit beim Aufheben und erneuten Melden; der Verlauf hält Quelle und Konto fest (#6842)",
		DependsOn: []string{
			schoolAbsenceReportsVersion, // the source check the clearing source mirrors
			mealAbsenceHistoryVersion,   // the history trigger this migration extends
		},
	})
	Migrations.MustRegister(statusDayProvenanceUp, statusDayProvenanceDown)
}

// statusDayProvenanceUpSQL keeps who reported and who cleared a status day
// (#6842). Clearing used to overwrite source, so a parent's report cleared by
// staff read as manual. Now the clearing goes to its own columns and source
// stays the report's. A withdrawn end-of-day archive is told apart from a
// standing one by its cleared_source.
//
// Rows cleared before this migration already lost their report source; their
// source is the clearing's, so it moves to cleared_source as well. The
// history trigger records the source and account of every report and
// clearing, so a later report no longer erases the first; its function lives
// in repeatable_schedule_record_meal_sickness_status.go (ADR 0223).
const statusDayProvenanceUpSQL = `
	ALTER TABLE active.student_status_days
		ADD COLUMN IF NOT EXISTS reported_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
		ADD COLUMN IF NOT EXISTS cleared_source TEXT,
		ADD COLUMN IF NOT EXISTS cleared_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL;
	ALTER TABLE active.student_status_days
		DROP CONSTRAINT IF EXISTS chk_student_status_days_cleared_source;
	ALTER TABLE active.student_status_days
		ADD CONSTRAINT chk_student_status_days_cleared_source
		CHECK (cleared_source IS NULL OR (cleared_at IS NOT NULL
			AND cleared_source IN ('manual', 'planned', 'next_checkin', 'end_of_day', 'parent', 'school')));
	CREATE INDEX IF NOT EXISTS idx_student_status_days_reported_by
		ON active.student_status_days (reported_by_account_id) WHERE reported_by_account_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_student_status_days_cleared_by
		ON active.student_status_days (cleared_by_account_id) WHERE cleared_by_account_id IS NOT NULL;

	UPDATE active.student_status_days
	SET cleared_source = source
	WHERE cleared_at IS NOT NULL AND source <> 'end_of_day' AND cleared_source IS NULL;

	ALTER TABLE schedule.meal_sickness_status_history
		ADD COLUMN IF NOT EXISTS source TEXT,
		ADD COLUMN IF NOT EXISTS account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL;
	CREATE INDEX IF NOT EXISTS idx_meal_sickness_history_account
		ON schedule.meal_sickness_status_history (account_id) WHERE account_id IS NOT NULL;
	COMMENT ON COLUMN schedule.meal_sickness_status_history.source IS
		'Quelle der Meldung oder des Aufhebens, das diesen Eintrag ausgelöst hat (#6842).';
	COMMENT ON COLUMN schedule.meal_sickness_status_history.account_id IS
		'Konto, das gemeldet oder aufgehoben hat; leer bei Geräten und automatischen Läufen (#6842).';

	DROP TRIGGER IF EXISTS record_meal_sickness_status ON active.student_status_days;
	CREATE TRIGGER record_meal_sickness_status
		AFTER INSERT OR UPDATE OF status, reported_at, cleared_at, source, reported_by_account_id ON active.student_status_days
		FOR EACH ROW EXECUTE FUNCTION schedule.record_meal_sickness_status();
`

// statusDayProvenanceDownSQL hands a withdrawn archive its clearing source
// back, so the earlier end_of_day test no longer reads it as standing, and
// restores the trigger's column list. The columns stay: the repeatable
// history function reads them, and a down no longer restores an earlier
// function (ADR 0223). Clear the retained clearing fields: earlier writers
// reopen days by clearing only cleared_at, which would violate the check.
// The history keeps the clearing's source and account.
const statusDayProvenanceDownSQL = `
	UPDATE active.student_status_days
	SET source = cleared_source
	WHERE source = 'end_of_day' AND cleared_source IS NOT NULL;

	UPDATE active.student_status_days
	SET cleared_source = NULL, cleared_by_account_id = NULL
	WHERE cleared_source IS NOT NULL OR cleared_by_account_id IS NOT NULL;

	DROP TRIGGER IF EXISTS record_meal_sickness_status ON active.student_status_days;
	CREATE TRIGGER record_meal_sickness_status
		AFTER INSERT OR UPDATE OF status, reported_at, cleared_at ON active.student_status_days
		FOR EACH ROW EXECUTE FUNCTION schedule.record_meal_sickness_status();
`

func statusDayProvenanceUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, statusDayProvenanceUpSQL); err != nil {
			return fmt.Errorf("keep the provenance of status days: %w", err)
		}
		return nil
	})
}

func statusDayProvenanceDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, statusDayProvenanceDownSQL); err != nil {
			return fmt.Errorf("restore the status day sources before the provenance: %w", err)
		}
		return nil
	})
}
