package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentsDepartureAccompaniedVersion     = "1.15.138"
	studentsDepartureAccompaniedDescription = "Allow 'accompanied' departure mode and add departure_companion_note to users.students"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentsDepartureAccompaniedVersion,
		Description: studentsDepartureAccompaniedDescription,
		DependsOn: []string{
			studentsAllowedDepartureModesVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentsDepartureAccompaniedUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentsDepartureAccompaniedDown(ctx, db)
		},
	)
}

func studentsDepartureAccompaniedUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.138: Allowing 'accompanied' departure mode + adding departure_companion_note...")

	// Widen the exclusive per-day validator to accept 'accompanied'. Replacing
	// the IMMUTABLE function is enough: the CHECK constraints reference it by
	// name and re-validate only on future writes, and widening the accepted set
	// can never invalidate an existing row.
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
						OR (elem.value #>> '{}') NOT IN ('alone', 'bus', 'pickup', 'accompanied')
				)
		$$;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed widening users.is_valid_departure_days: %w", err)
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
							WHERE mode.value NOT IN ('alone', 'bus', 'pickup', 'accompanied')
						)
						OR (
							SELECT count(*) <> count(DISTINCT mode.value)
							FROM jsonb_array_elements_text(elem.value) AS mode(value)
						)
				)
		$$;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed widening users.is_valid_allowed_departure_modes: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			ADD COLUMN IF NOT EXISTS departure_companion_note TEXT;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding users.students.departure_companion_note: %w", err)
	}

	return nil
}

func studentsDepartureAccompaniedDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.138: Removing 'accompanied' departure mode + departure_companion_note...")

	if _, err := db.NewRaw(`
		ALTER TABLE users.students
			DROP COLUMN IF EXISTS departure_companion_note;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping users.students.departure_companion_note: %w", err)
	}

	// Restore the narrower validators. Rows already carrying 'accompanied' are
	// not re-checked, but any future write of that value will be rejected again.
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
		return fmt.Errorf("failed restoring users.is_valid_departure_days: %w", err)
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
		return fmt.Errorf("failed restoring users.is_valid_allowed_departure_modes: %w", err)
	}

	return nil
}
