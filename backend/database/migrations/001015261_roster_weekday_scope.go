package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	rosterWeekdayScopeVersion     = "1.15.261"
	rosterWeekdayScopeDescription = "Scope activity roster rows (staff and children) to a single weekday (issue #2129)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     rosterWeekdayScopeVersion,
		Description: rosterWeekdayScopeDescription,
		DependsOn:   []string{backfillMensaCategoryVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error { return rosterWeekdayScopeUp(ctx, db) },
		func(ctx context.Context, db *bun.DB) error { return rosterWeekdayScopeDown(ctx, db) },
	)
}

// rosterWeekdayScopeUp adds a nullable ISO weekday (1=Mon … 7=Sun) to both
// roster tables of a recurring template.
//
// NULL means "applies on every weekday the series runs" — which is exactly
// what every existing row means today, so no backfill is needed and the
// behaviour of untouched templates is unchanged (issue #2129 AC6).
//
// A non-NULL value scopes the row to that one weekday. The template writer
// expands the shared default plus the per-weekday deviations into concrete
// per-weekday rows, so readers only ever need the trivial predicate
// `weekday IS NULL OR weekday = ISODOW(date)`. That keeps materialization,
// capacity scoring and the conflict/coverage probes on one rule.
//
// Deliberately NOT reusing student_enrollments.selected_weekdays: that column
// is owned by the enrollment/care-offering decision path, and the template
// editor treats rows carrying it as foreign and preserves them untouched
// (services/schedule/template_update_service.go). Overloading it would make
// editor-owned and offer-owned rows indistinguishable.
func rosterWeekdayScopeUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.261: Adding weekday scope to activity roster rows...")

	if _, err := db.NewRaw(`
		ALTER TABLE activities.supervisors
			ADD COLUMN IF NOT EXISTS weekday SMALLINT;

		ALTER TABLE activities.supervisors
			DROP CONSTRAINT IF EXISTS chk_activities_supervisors_weekday,
			ADD CONSTRAINT chk_activities_supervisors_weekday
				CHECK (weekday IS NULL OR (weekday >= 1 AND weekday <= 7));

		CREATE INDEX IF NOT EXISTS idx_activities_supervisors_group_weekday
			ON activities.supervisors (tenant_id, group_id, weekday);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding weekday to activities.supervisors: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE activities.student_enrollments
			ADD COLUMN IF NOT EXISTS weekday SMALLINT;

		ALTER TABLE activities.student_enrollments
			DROP CONSTRAINT IF EXISTS chk_activities_student_enrollments_weekday,
			ADD CONSTRAINT chk_activities_student_enrollments_weekday
				CHECK (weekday IS NULL OR (weekday >= 1 AND weekday <= 7));

		CREATE INDEX IF NOT EXISTS idx_activities_student_enrollments_group_weekday
			ON activities.student_enrollments (tenant_id, activity_group_id, weekday);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding weekday to activities.student_enrollments: %w", err)
	}

	// The active-roster uniqueness indexes (migration 1.15.52) key on
	// (tenant, person, group, period). A child who attends on Monday AND
	// Tuesday now has one row per weekday, so the weekday scope has to be part
	// of the key or the second row collides. COALESCE(weekday, 0) keeps the
	// legacy series-wide rows on a single slot, so the constraint they enforce
	// today is unchanged.
	if _, err := db.NewRaw(`
		DROP INDEX IF EXISTS activities.idx_student_enrollments_active;
		CREATE UNIQUE INDEX idx_student_enrollments_active
		ON activities.student_enrollments (
			tenant_id,
			student_id,
			activity_group_id,
			COALESCE(calendar_period_id, 0),
			COALESCE(weekday, 0)
		)
		WHERE valid_until IS NULL;

		DROP INDEX IF EXISTS activities.idx_supervisors_active;
		CREATE UNIQUE INDEX idx_supervisors_active
		ON activities.supervisors (
			tenant_id,
			staff_id,
			group_id,
			COALESCE(calendar_period_id, 0),
			COALESCE(weekday, 0)
		)
		WHERE valid_until IS NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed rebuilding weekday-scoped roster unique indexes: %w", err)
	}

	// The "one primary supervisor per group" trigger predates per-weekday
	// staffing: it clears is_primary on EVERY other row of the group, so a
	// Tuesday "Zuständige Person" would silently unset the Monday one. Scope
	// it to the calendar period and weekday, NULL-safely — with every row on
	// the legacy NULL scope the behaviour is byte-for-byte what it was before.
	if _, err := db.NewRaw(`
		CREATE OR REPLACE FUNCTION activities.ensure_single_primary_supervisor()
		RETURNS TRIGGER AS $$
		BEGIN
			IF NEW.is_primary = TRUE THEN
				UPDATE activities.supervisors
				SET is_primary = FALSE
				WHERE group_id = NEW.group_id
					AND id <> NEW.id
					AND calendar_period_id IS NOT DISTINCT FROM NEW.calendar_period_id
					AND weekday IS NOT DISTINCT FROM NEW.weekday;
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed scoping the primary supervisor trigger to calendar period and weekday: %w", err)
	}

	return nil
}

func rosterWeekdayScopeDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.261: Removing weekday scope from activity roster rows...")

	// The old indexes cannot represent several active weekday rows for the same
	// person, group, and period. Collapse those rows first so rollback remains
	// executable after weekday-specific rosters have been used. Prefer a legacy
	// series-wide row when one exists; for supervisors, otherwise keep a primary
	// row. Closed history remains untouched.
	if _, err := db.NewRaw(`
		WITH ranked AS (
			SELECT id,
				ROW_NUMBER() OVER (
					PARTITION BY tenant_id, student_id, activity_group_id, COALESCE(calendar_period_id, 0)
					ORDER BY (weekday IS NULL) DESC, id ASC
				) AS position
			FROM activities.student_enrollments
			WHERE valid_until IS NULL
		), duplicates AS (
			SELECT id FROM ranked WHERE position > 1
		)
		DELETE FROM activities.student_enrollments AS enrollment
		USING duplicates
		WHERE enrollment.id = duplicates.id;

		WITH ranked AS (
			SELECT id,
				ROW_NUMBER() OVER (
					PARTITION BY tenant_id, staff_id, group_id, COALESCE(calendar_period_id, 0)
					ORDER BY (weekday IS NULL) DESC, is_primary DESC, id ASC
				) AS position
			FROM activities.supervisors
			WHERE valid_until IS NULL
		), duplicates AS (
			SELECT id FROM ranked WHERE position > 1
		)
		DELETE FROM activities.supervisors AS supervisor
		USING duplicates
		WHERE supervisor.id = duplicates.id;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed collapsing weekday-scoped roster rows: %w", err)
	}

	// Restore the period-scoped (weekday-agnostic) uniqueness of migration
	// 1.15.52 before the column disappears.
	if _, err := db.NewRaw(`
		DROP INDEX IF EXISTS activities.idx_student_enrollments_active;
		CREATE UNIQUE INDEX idx_student_enrollments_active
		ON activities.student_enrollments (
			tenant_id,
			student_id,
			activity_group_id,
			COALESCE(calendar_period_id, 0)
		)
		WHERE valid_until IS NULL;

		DROP INDEX IF EXISTS activities.idx_supervisors_active;
		CREATE UNIQUE INDEX idx_supervisors_active
		ON activities.supervisors (
			tenant_id,
			staff_id,
			group_id,
			COALESCE(calendar_period_id, 0)
		)
		WHERE valid_until IS NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed restoring period-scoped roster unique indexes: %w", err)
	}

	// Restore the group-wide primary trigger before the column disappears.
	if _, err := db.NewRaw(`
		CREATE OR REPLACE FUNCTION activities.ensure_single_primary_supervisor()
		RETURNS TRIGGER AS $$
		BEGIN
			IF NEW.is_primary = TRUE THEN
				UPDATE activities.supervisors
				SET is_primary = FALSE
				WHERE group_id = NEW.group_id
					AND id != NEW.id;
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed restoring the group-wide primary supervisor trigger: %w", err)
	}

	if _, err := db.NewRaw(`
		DROP INDEX IF EXISTS activities.idx_activities_student_enrollments_group_weekday;
		ALTER TABLE activities.student_enrollments
			DROP CONSTRAINT IF EXISTS chk_activities_student_enrollments_weekday,
			DROP COLUMN IF EXISTS weekday;

		DROP INDEX IF EXISTS activities.idx_activities_supervisors_group_weekday;
		ALTER TABLE activities.supervisors
			DROP CONSTRAINT IF EXISTS chk_activities_supervisors_weekday,
			DROP COLUMN IF EXISTS weekday;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing weekday from activity roster tables: %w", err)
	}
	return nil
}
