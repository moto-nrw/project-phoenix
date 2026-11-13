package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const statusDayProvenanceVersion = "1.15.964"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     statusDayProvenanceVersion,
		Description: "Status-Tage behalten Quelle und Meldezeit beim Aufheben und erneuten Melden; der Verlauf hält Quelle und Konto fest (#6842)",
		DependsOn: []string{
			traegerExportRecipientsVersion, // the newest migration on development (1.15.963)
			schoolAbsenceReportsVersion,    // the source check the clearing source mirrors
			mealAbsenceHistoryVersion,      // the history trigger this migration extends
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
// clearing, so a later report no longer erases the first.
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

	CREATE OR REPLACE FUNCTION schedule.record_meal_sickness_status()
	RETURNS TRIGGER AS $$
	DECLARE
		effective_change TIMESTAMPTZ;
		event_source TEXT;
		event_account BIGINT;
	BEGIN
		IF TG_OP = 'DELETE' AND current_setting('app.retention_purge', true) = 'on' THEN
			RETURN OLD;
		END IF;

		IF TG_OP = 'DELETE' THEN
			effective_change := clock_timestamp();
			INSERT INTO schedule.meal_sickness_status_history
				(tenant_id, student_id, date, status, changed_at, reported_at, cleared_at)
			VALUES
				(OLD.tenant_id, OLD.student_id, OLD.date, OLD.status, effective_change, OLD.reported_at, effective_change);
			RETURN OLD;
		END IF;

		IF TG_OP = 'UPDATE' AND OLD.status <> NEW.status THEN
			effective_change := clock_timestamp();
			INSERT INTO schedule.meal_sickness_status_history
				(tenant_id, student_id, date, status, changed_at, reported_at, cleared_at)
			VALUES
				(OLD.tenant_id, OLD.student_id, OLD.date, OLD.status, effective_change, OLD.reported_at, effective_change);
		ELSIF TG_OP = 'INSERT' THEN
			effective_change := NEW.reported_at;
		ELSIF NEW.reported_at IS DISTINCT FROM OLD.reported_at THEN
			effective_change := NEW.reported_at;
		ELSIF NEW.cleared_at IS DISTINCT FROM OLD.cleared_at THEN
			effective_change := COALESCE(NEW.cleared_at, clock_timestamp());
		ELSE
			RETURN NEW;
		END IF;

		IF NEW.cleared_source IS NOT NULL THEN
			event_source := NEW.cleared_source;
			event_account := NEW.cleared_by_account_id;
		ELSE
			event_source := NEW.source;
			event_account := NEW.reported_by_account_id;
		END IF;

		INSERT INTO schedule.meal_sickness_status_history
			(tenant_id, student_id, date, status, changed_at, reported_at, cleared_at, source, account_id)
		VALUES
			(NEW.tenant_id, NEW.student_id, NEW.date, NEW.status, effective_change, NEW.reported_at, NEW.cleared_at, event_source, event_account);
		RETURN NEW;
	END;
	$$ LANGUAGE plpgsql;
`

// statusDayProvenanceDownSQL hands a withdrawn archive its clearing source
// back, so the earlier end_of_day test no longer reads it as standing,
// then drops the columns. The trigger goes back first; it reads them.
const statusDayProvenanceDownSQL = `
	UPDATE active.student_status_days
	SET source = cleared_source
	WHERE source = 'end_of_day' AND cleared_source IS NOT NULL;

	ALTER TABLE schedule.meal_sickness_status_history
		DROP COLUMN IF EXISTS account_id,
		DROP COLUMN IF EXISTS source;
	ALTER TABLE active.student_status_days
		DROP CONSTRAINT IF EXISTS chk_student_status_days_cleared_source,
		DROP COLUMN IF EXISTS cleared_by_account_id,
		DROP COLUMN IF EXISTS cleared_source,
		DROP COLUMN IF EXISTS reported_by_account_id;
`

// statusDayProvenancePreviousTriggerSQL is the history trigger of 1.15.600,
// which the down migration restores before it drops the columns it reads.
const statusDayProvenancePreviousTriggerSQL = `
	CREATE OR REPLACE FUNCTION schedule.record_meal_sickness_status()
	RETURNS TRIGGER AS $$
	DECLARE
		effective_change TIMESTAMPTZ;
	BEGIN
		IF TG_OP = 'DELETE' AND current_setting('app.retention_purge', true) = 'on' THEN
			RETURN OLD;
		END IF;

		IF TG_OP = 'DELETE' THEN
			effective_change := clock_timestamp();
			INSERT INTO schedule.meal_sickness_status_history
				(tenant_id, student_id, date, status, changed_at, reported_at, cleared_at)
			VALUES
				(OLD.tenant_id, OLD.student_id, OLD.date, OLD.status, effective_change, OLD.reported_at, effective_change);
			RETURN OLD;
		END IF;

		IF TG_OP = 'UPDATE' AND OLD.status <> NEW.status THEN
			effective_change := clock_timestamp();
			INSERT INTO schedule.meal_sickness_status_history
				(tenant_id, student_id, date, status, changed_at, reported_at, cleared_at)
			VALUES
				(OLD.tenant_id, OLD.student_id, OLD.date, OLD.status, effective_change, OLD.reported_at, effective_change);
		ELSIF TG_OP = 'INSERT' THEN
			effective_change := NEW.reported_at;
		ELSIF NEW.reported_at IS DISTINCT FROM OLD.reported_at THEN
			effective_change := NEW.reported_at;
		ELSIF NEW.cleared_at IS DISTINCT FROM OLD.cleared_at THEN
			effective_change := COALESCE(NEW.cleared_at, clock_timestamp());
		ELSE
			RETURN NEW;
		END IF;

		INSERT INTO schedule.meal_sickness_status_history
			(tenant_id, student_id, date, status, changed_at, reported_at, cleared_at)
		VALUES
			(NEW.tenant_id, NEW.student_id, NEW.date, NEW.status, effective_change, NEW.reported_at, NEW.cleared_at);
		RETURN NEW;
	END;
	$$ LANGUAGE plpgsql;
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
		if _, err := tx.ExecContext(ctx, statusDayProvenancePreviousTriggerSQL); err != nil {
			return fmt.Errorf("restore the meal absence history trigger: %w", err)
		}
		if _, err := tx.ExecContext(ctx, statusDayProvenanceDownSQL); err != nil {
			return fmt.Errorf("drop the provenance of status days: %w", err)
		}
		return nil
	})
}
