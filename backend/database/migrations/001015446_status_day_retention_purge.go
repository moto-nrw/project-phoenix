package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	statusDayRetentionPurgeVersion     = "1.15.446"
	statusDayRetentionPurgeDescription = "Fristlöschung von Krank-Tagen schreibt keine neue Zeile in die Essens-Krankhistorie (#3958)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     statusDayRetentionPurgeVersion,
		Description: statusDayRetentionPurgeDescription,
		DependsOn:   []string{mealParticipationVersion, dataCleanupSwitchOperatorOnlyVersion},
	})

	Migrations.MustRegister(statusDayRetentionPurgeUp, statusDayRetentionPurgeDown)
}

// The trigger function of 1.15.365, split so the retention variant differs
// only in its first check.
const (
	mealSicknessStatusHead = `
		CREATE OR REPLACE FUNCTION schedule.record_meal_sickness_status()
		RETURNS TRIGGER AS $$
		DECLARE
			effective_change TIMESTAMPTZ;
		BEGIN
`
	mealSicknessStatusRetentionSkip = `
			IF TG_OP = 'DELETE' AND current_setting('app.retention_purge', true) = 'on' THEN
				RETURN OLD;
			END IF;
`
	mealSicknessStatusBody = `
			IF TG_OP = 'DELETE' THEN
				IF OLD.status <> 'sick' THEN
					RETURN OLD;
				END IF;
				effective_change := clock_timestamp();
				INSERT INTO schedule.meal_sickness_status_history
					(tenant_id, student_id, date, changed_at, reported_at, cleared_at)
				VALUES
					(OLD.tenant_id, OLD.student_id, OLD.date, effective_change, OLD.reported_at, effective_change);
				RETURN OLD;
			END IF;

			IF TG_OP = 'UPDATE' AND OLD.status = 'sick' AND NEW.status <> 'sick' THEN
				effective_change := clock_timestamp();
				INSERT INTO schedule.meal_sickness_status_history
					(tenant_id, student_id, date, changed_at, reported_at, cleared_at)
				VALUES
					(OLD.tenant_id, OLD.student_id, OLD.date, effective_change, OLD.reported_at, effective_change);
				RETURN NEW;
			END IF;

			IF NEW.status <> 'sick' THEN
				RETURN NEW;
			END IF;
			IF TG_OP = 'INSERT' THEN
				effective_change := NEW.reported_at;
			ELSIF OLD.status <> 'sick' OR NEW.reported_at IS DISTINCT FROM OLD.reported_at THEN
				effective_change := NEW.reported_at;
			ELSIF NEW.cleared_at IS DISTINCT FROM OLD.cleared_at THEN
				effective_change := COALESCE(NEW.cleared_at, clock_timestamp());
			ELSE
				RETURN NEW;
			END IF;

			INSERT INTO schedule.meal_sickness_status_history
				(tenant_id, student_id, date, changed_at, reported_at, cleared_at)
			VALUES
				(NEW.tenant_id, NEW.student_id, NEW.date, effective_change, NEW.reported_at, NEW.cleared_at);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
`
	mealSicknessStatusRetentionSQL = mealSicknessStatusHead + mealSicknessStatusRetentionSkip + mealSicknessStatusBody
	mealSicknessStatusOriginalSQL  = mealSicknessStatusHead + mealSicknessStatusBody
)

// statusDayRetentionPurgeUp lets the nightly retention delete expired status
// days without the meal plan's trigger recording each deleted sick day as
// "cleared now". That record would copy the health datum into a new row and
// mark a long-billed day as no longer sick. The Care Plan purge sets
// app.retention_purge for its own statement; every other delete still
// records the clearance exactly as before.
func statusDayRetentionPurgeUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, mealSicknessStatusRetentionSQL); err != nil {
		return fmt.Errorf("let the status day retention skip the meal sickness history: %w", err)
	}
	return nil
}

func statusDayRetentionPurgeDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, mealSicknessStatusOriginalSQL); err != nil {
		return fmt.Errorf("restore the meal sickness history trigger: %w", err)
	}
	return nil
}
