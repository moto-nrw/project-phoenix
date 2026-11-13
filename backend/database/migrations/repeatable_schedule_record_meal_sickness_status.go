package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	RegisterRepeatable("schedule.record_meal_sickness_status", func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, repeatableScheduleRecordMealSicknessStatusSQL)
		return err
	})
}

// repeatableScheduleRecordMealSicknessStatusSQL is the current definition of
// schedule.record_meal_sickness_status() (ADR 0223). Change the function
// here; numbered migrations no longer define it.
//
// It records every absence status of a day for the meal plan, with the
// source and account of each report and clearing (#6842). The retention
// purge records nothing.
const repeatableScheduleRecordMealSicknessStatusSQL = `CREATE OR REPLACE FUNCTION schedule.record_meal_sickness_status()
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
		ELSIF NEW.source IS DISTINCT FROM OLD.source
			OR NEW.reported_by_account_id IS DISTINCT FROM OLD.reported_by_account_id THEN
			-- A new reporter on a standing day keeps its report time (a
			-- profile note update reuses the flag's since time).
			effective_change := clock_timestamp();
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
