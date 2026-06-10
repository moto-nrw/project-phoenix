package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentsDepartureDaysVersion     = "1.15.120"
	studentsDepartureDaysDescription = "Add unified per-weekday departure mode (alone/bus/pickup) to users.students"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentsDepartureDaysVersion,
		Description: studentsDepartureDaysDescription,
		DependsOn: []string{
			studentsPickupDaysVersion,
			studentsDropBusColumnVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentsDepartureDaysUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentsDepartureDaysDown(ctx, db)
		},
	)
}

func studentsDepartureDaysUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.120: Adding departure_days to users.students...")

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			ADD COLUMN IF NOT EXISTS departure_days JSONB NOT NULL DEFAULT '{}'::jsonb;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding departure_days to users.students: %w", err)
	}

	// Backfill the unified enum from the two legacy boolean maps. Pickup wins
	// when a day is flagged as both bus and pickup — matching
	// users.DepartureDaysFromLegacy — so contradictory pre-unification rows
	// resolve to the higher-stakes "picked up" instruction.
	if _, err := db.NewRaw(`
		UPDATE users.students AS s
		SET departure_days = (
			SELECT COALESCE(jsonb_object_agg(d.day, x.mode), '{}'::jsonb)
			FROM (VALUES ('mon'), ('tue'), ('wed'), ('thu'), ('fri')) AS d(day)
			CROSS JOIN LATERAL (
				SELECT CASE
					WHEN COALESCE((s.pickup_days ->> d.day)::boolean, FALSE) THEN 'pickup'
					WHEN COALESCE((s.bus_days ->> d.day)::boolean, FALSE) THEN 'bus'
				END AS mode
			) AS x
			WHERE x.mode IS NOT NULL
		)
		WHERE departure_days = '{}'::jsonb;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed backfilling users.students.departure_days: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE OR REPLACE FUNCTION users.is_valid_departure_days(value JSONB)
		RETURNS BOOLEAN
		LANGUAGE sql
		IMMUTABLE
		AS $$
			SELECT jsonb_typeof(value) = 'object'
				AND NOT EXISTS (
					SELECT 1
					FROM jsonb_each(value) AS elem(key, value)
					WHERE elem.key NOT IN ('mon', 'tue', 'wed', 'thu', 'fri')
						OR jsonb_typeof(elem.value) <> 'string'
						OR (elem.value #>> '{}') NOT IN ('alone', 'bus', 'pickup')
				)
		$$;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating users.students departure_days validation function: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			DROP CONSTRAINT IF EXISTS check_students_departure_days,
			ADD CONSTRAINT check_students_departure_days CHECK (users.is_valid_departure_days(departure_days));
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding users.students.departure_days check constraint: %w", err)
	}

	return nil
}

func studentsDepartureDaysDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.120: Removing departure_days from users.students...")

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			DROP CONSTRAINT IF EXISTS check_students_departure_days,
			DROP COLUMN IF EXISTS departure_days;
		DROP FUNCTION IF EXISTS users.is_valid_departure_days(JSONB);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing users.students.departure_days: %w", err)
	}
	return nil
}
