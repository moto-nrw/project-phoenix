package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	mealAbsenceHistoryVersion     = "1.15.500"
	mealAbsenceHistoryDescription = "Mittagessen: entschuldigt und Ausflug melden das Essen ab wie eine Krankmeldung (#3873)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mealAbsenceHistoryVersion,
		Description: mealAbsenceHistoryDescription,
		DependsOn: []string{
			statusDayRetentionPurgeVersion, // the trigger function this one replaces
			mealPlanAllergensVersion,       // development's latest migrations: meal plan allergens,
			"1.15.499",                     // password login protection
			morningAttendanceVersion,       // and morning attendance
		},
	})
	Migrations.MustRegister(mealAbsenceHistoryUp, mealAbsenceHistoryDown)
}

// mealAbsenceHistorySQL records every absence status of a day, not only
// sick: an excused child or one on a class trip does not eat either. Each
// row carries its status, so a cleared status never hides another one of the
// same day. The retention purge of 1.15.446 still records nothing.
const mealAbsenceHistorySQL = `
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

func mealAbsenceHistoryUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE schedule.meal_sickness_status_history
				ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'sick';
			ALTER TABLE schedule.meal_sickness_status_history
				DROP CONSTRAINT IF EXISTS chk_meal_absence_history_status;
			ALTER TABLE schedule.meal_sickness_status_history
				ADD CONSTRAINT chk_meal_absence_history_status CHECK (status IN ('sick', 'excused', 'class_trip'));
			COMMENT ON TABLE schedule.meal_sickness_status_history IS
				'Verlauf der Abwesenheiten (krank, entschuldigt, Ausflug) je Kind und Tag, damit die Essensbestellung den Stand zur Frist kennt.';
			DROP INDEX IF EXISTS schedule.idx_meal_sickness_history_cutoff;
			CREATE INDEX idx_meal_sickness_history_cutoff
				ON schedule.meal_sickness_status_history (tenant_id, date, student_id, status, changed_at DESC, id DESC);
		`); err != nil {
			return fmt.Errorf("add status to the meal absence history: %w", err)
		}
		if _, err := tx.ExecContext(ctx, mealAbsenceHistorySQL); err != nil {
			return fmt.Errorf("record every absence status for the meal plan: %w", err)
		}
		// Carry the open and cleared excused and class trip days over, as
		// 1.15.365 did for the sick days. A rerun finds them already there.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO schedule.meal_sickness_status_history
				(tenant_id, student_id, date, status, changed_at, reported_at, cleared_at)
			SELECT day.tenant_id, day.student_id, day.date, day.status, day.reported_at, day.reported_at, NULL
			FROM active.student_status_days AS day
			WHERE day.status IN ('excused', 'class_trip')
				AND NOT EXISTS (
					SELECT 1 FROM schedule.meal_sickness_status_history AS history
					WHERE history.tenant_id = day.tenant_id AND history.student_id = day.student_id
						AND history.date = day.date AND history.status = day.status
				);
			INSERT INTO schedule.meal_sickness_status_history
				(tenant_id, student_id, date, status, changed_at, reported_at, cleared_at)
			SELECT day.tenant_id, day.student_id, day.date, day.status, day.cleared_at, day.reported_at, day.cleared_at
			FROM active.student_status_days AS day
			WHERE day.status IN ('excused', 'class_trip') AND day.cleared_at IS NOT NULL
				AND NOT EXISTS (
					SELECT 1 FROM schedule.meal_sickness_status_history AS history
					WHERE history.tenant_id = day.tenant_id AND history.student_id = day.student_id
						AND history.date = day.date AND history.status = day.status
						AND history.cleared_at IS NOT NULL
				);
		`); err != nil {
			return fmt.Errorf("carry excused and class trip days into the meal absence history: %w", err)
		}
		return nil
	})
}

func mealAbsenceHistoryDown(ctx context.Context, db *bun.DB) error {
	// The trigger of 1.15.446 records sick days only and never reads the
	// status column, so it goes back first.
	if err := statusDayRetentionPurgeUp(ctx, db); err != nil {
		return fmt.Errorf("restore the sick-only meal history trigger: %w", err)
	}
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM schedule.meal_sickness_status_history WHERE status <> 'sick';
			DROP INDEX IF EXISTS schedule.idx_meal_sickness_history_cutoff;
			CREATE INDEX idx_meal_sickness_history_cutoff
				ON schedule.meal_sickness_status_history (tenant_id, date, student_id, changed_at DESC, id DESC);
			ALTER TABLE schedule.meal_sickness_status_history DROP CONSTRAINT IF EXISTS chk_meal_absence_history_status;
			ALTER TABLE schedule.meal_sickness_status_history DROP COLUMN IF EXISTS status;
			COMMENT ON TABLE schedule.meal_sickness_status_history IS NULL;
		`); err != nil {
			return fmt.Errorf("drop the status of the meal absence history: %w", err)
		}
		return nil
	})
}
