package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// 1.15.39 — backfill the iteration-4 / E15 promise that was never delivered:
// every existing tenant should have an active "Schuljahr Y/Y+1" calendar
// period so the materialization service has an anchor on its first run.
//
// New tenants get this from operatorProvisioningService.seedDefaultCalendarPeriod
// (added in the same PR). This migration handles existing tenants without
// any active period — typically just the demo schools and any production
// tenants provisioned before the seed hook landed.
//
// The school year is derived from CURRENT_DATE on the migration host:
//   - month >= 8  → "Schuljahr Y/Y+1", Aug 1 of Y to Jul 31 of Y+1
//   - month <  8  → "Schuljahr Y-1/Y", Aug 1 of Y-1 to Jul 31 of Y
//
// Idempotent: WHERE NOT EXISTS skips tenants that already have any active
// period, regardless of name. Re-running this migration will not produce
// duplicates.
const (
	backfillDefaultCalendarPeriodsVersion     = "1.15.39"
	backfillDefaultCalendarPeriodsDescription = "Backfill default school year calendar periods for tenants missing one"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     backfillDefaultCalendarPeriodsVersion,
		Description: backfillDefaultCalendarPeriodsDescription,
		DependsOn: []string{
			createCalendarPeriodsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return backfillDefaultCalendarPeriodsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return backfillDefaultCalendarPeriodsDown(ctx, db)
		},
	)
}

func backfillDefaultCalendarPeriodsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.39: Backfilling default calendar periods for tenants without one...")

	// Single SQL statement: for every school with no active calendar period,
	// insert one school_year period covering the current school year.
	//
	// Migrations connect as the postgres superuser (per CLAUDE.md), which
	// bypasses RLS automatically — no app.current_tenant_id dance needed.
	_, err := db.ExecContext(ctx, `
		WITH bounds AS (
			SELECT
				CASE
					WHEN EXTRACT(MONTH FROM CURRENT_DATE) >= 8
						THEN EXTRACT(YEAR FROM CURRENT_DATE)::int
					ELSE (EXTRACT(YEAR FROM CURRENT_DATE) - 1)::int
				END AS start_year
		),
		derived AS (
			SELECT
				start_year,
				start_year + 1 AS end_year
			FROM bounds
		)
		INSERT INTO schedule.calendar_periods
			(tenant_id, name, period_type, start_date, end_date,
			 week_cycle_length, is_active, created_at, updated_at)
		SELECT
			s.id,
			'Schuljahr ' || d.start_year || '/' || d.end_year,
			'school_year',
			MAKE_DATE(d.start_year, 8, 1),
			MAKE_DATE(d.end_year, 7, 31),
			1,
			true,
			NOW(),
			NOW()
		FROM platform.schools s
		CROSS JOIN derived d
		WHERE NOT EXISTS (
			SELECT 1
			FROM schedule.calendar_periods p
			WHERE p.tenant_id = s.id AND p.is_active = true
		)
	`)
	if err != nil {
		return fmt.Errorf("backfill default calendar periods: %w", err)
	}

	return nil
}

func backfillDefaultCalendarPeriodsDown(ctx context.Context, db *bun.DB) error {
	// Intentional no-op: tenants may have customised the period (renamed,
	// shifted dates) before a rollback runs. Removing periods we did not
	// uniquely tag would discard real admin work. Reverting this migration
	// just leaves the seeded rows in place.
	fmt.Println("Rolling back migration 1.15.39: no-op (default periods preserved)")
	return nil
}
