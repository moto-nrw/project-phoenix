package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentsAllowedDepartureModesVersion     = "1.15.130"
	studentsAllowedDepartureModesDescription = "Add multi-select allowed departure modes to users.students"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentsAllowedDepartureModesVersion,
		Description: studentsAllowedDepartureModesDescription,
		DependsOn: []string{
			studentsDepartureDaysVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentsAllowedDepartureModesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentsAllowedDepartureModesDown(ctx, db)
		},
	)
}

func studentsAllowedDepartureModesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.130: Adding allowed_departure_modes to users.students...")

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			ADD COLUMN IF NOT EXISTS allowed_departure_modes JSONB NOT NULL DEFAULT '{}'::jsonb;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding allowed_departure_modes to users.students: %w", err)
	}

	if _, err := db.NewRaw(`
		UPDATE users.students AS s
		SET allowed_departure_modes = (
			SELECT COALESCE(jsonb_object_agg(backfill.day, backfill.modes), '{}'::jsonb)
			FROM (
				SELECT d.day,
					CASE
						WHEN s.bus_days @> jsonb_build_object(d.day, true)
							OR s.pickup_days @> jsonb_build_object(d.day, true)
						THEN (
							SELECT jsonb_agg(mode.value ORDER BY mode.ord)
							FROM (
								VALUES
									(1, 'bus'::text, s.bus_days @> jsonb_build_object(d.day, true)),
									(2, 'pickup'::text, s.pickup_days @> jsonb_build_object(d.day, true))
							) AS mode(ord, value, enabled)
							WHERE mode.enabled
						)
						WHEN s.departure_days ? d.day
							AND s.departure_days ->> d.day IN ('alone', 'bus', 'pickup')
						THEN jsonb_build_array(s.departure_days ->> d.day)
						ELSE NULL
					END AS modes
				FROM (VALUES ('mon'), ('tue'), ('wed'), ('thu'), ('fri')) AS d(day)
			) AS backfill
			WHERE backfill.modes IS NOT NULL
		)
		WHERE allowed_departure_modes = '{}'::jsonb
			AND (
				bus_days <> '{}'::jsonb
				OR pickup_days <> '{}'::jsonb
				OR departure_days <> '{}'::jsonb
			);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed backfilling users.students.allowed_departure_modes: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE OR REPLACE FUNCTION users.is_valid_allowed_departure_modes(value JSONB)
		RETURNS BOOLEAN
		LANGUAGE sql
		IMMUTABLE
		AS $$
			SELECT jsonb_typeof(value) = 'object'
				AND NOT EXISTS (
					SELECT 1
					FROM jsonb_each(value) AS elem(key, value)
					WHERE elem.key NOT IN ('mon', 'tue', 'wed', 'thu', 'fri')
						OR jsonb_typeof(elem.value) <> 'array'
						OR jsonb_array_length(elem.value) = 0
						OR EXISTS (
							SELECT 1
							FROM jsonb_array_elements_text(elem.value) AS mode(value)
							WHERE mode.value NOT IN ('alone', 'bus', 'pickup')
						)
						OR (
							SELECT count(*) <> count(DISTINCT mode.value)
							FROM jsonb_array_elements_text(elem.value) AS mode(value)
						)
				)
		$$;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating users.students allowed_departure_modes validation function: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			DROP CONSTRAINT IF EXISTS check_students_allowed_departure_modes,
			ADD CONSTRAINT check_students_allowed_departure_modes CHECK (users.is_valid_allowed_departure_modes(allowed_departure_modes));
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding users.students.allowed_departure_modes check constraint: %w", err)
	}

	return nil
}

func studentsAllowedDepartureModesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.130: Removing allowed_departure_modes from users.students...")

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			DROP CONSTRAINT IF EXISTS check_students_allowed_departure_modes,
			DROP COLUMN IF EXISTS allowed_departure_modes;
		DROP FUNCTION IF EXISTS users.is_valid_allowed_departure_modes(JSONB);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing users.students.allowed_departure_modes: %w", err)
	}
	return nil
}
