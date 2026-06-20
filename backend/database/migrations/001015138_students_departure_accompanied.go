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

	// Normalize persisted 'accompanied' values BEFORE narrowing the validators.
	// PostgreSQL re-evaluates a CHECK constraint against the whole new tuple on
	// every UPDATE (no "constrained column unchanged" optimization — that only
	// exists for FK triggers), so leaving 'accompanied' in place would make any
	// later edit to an affected student fail the restored CHECK. We downgrade
	// 'accompanied' to 'pickup' rather than dropping it: an accompanied child
	// ("geht mit anderem Kind/Person") leaves with a person, so 'pickup' ("wird
	// abgeholt") preserves the safety instruction — mapping to 'alone' would mark
	// a child who must not leave unaccompanied as going home alone. These updates
	// run while the wide validators are still active and only write 'pickup',
	// which both the wide and the restored narrow validators accept.
	if _, err := db.NewRaw(`
		UPDATE users.students
		SET departure_days = COALESCE((
			SELECT jsonb_object_agg(
				elem.key,
				CASE WHEN elem.value #>> '{}' = 'accompanied'
					THEN '"pickup"'::jsonb
					ELSE elem.value
				END
			)
			FROM jsonb_each(departure_days) AS elem(key, value)
		), '{}'::jsonb)
		WHERE EXISTS (
			SELECT 1
			FROM jsonb_each_text(departure_days) AS e(key, value)
			WHERE e.value = 'accompanied'
		);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed normalizing 'accompanied' in users.students.departure_days: %w", err)
	}

	// allowed_departure_modes is a per-day array; replace 'accompanied' with
	// 'pickup' and de-duplicate, because a day may already allow 'pickup' and the
	// restored validator rejects duplicate modes. Replacing (never removing)
	// guarantees no array is emptied, which the validator also rejects.
	if _, err := db.NewRaw(`
		UPDATE users.students
		SET allowed_departure_modes = COALESCE((
			SELECT jsonb_object_agg(day_modes.key, day_modes.modes)
			FROM (
				SELECT elem.key,
					(
						SELECT jsonb_agg(DISTINCT
							CASE WHEN m.value = 'accompanied' THEN 'pickup' ELSE m.value END
						)
						FROM jsonb_array_elements_text(elem.value) AS m(value)
					) AS modes
				FROM jsonb_each(allowed_departure_modes) AS elem(key, value)
			) AS day_modes
		), '{}'::jsonb)
		WHERE EXISTS (
			SELECT 1
			FROM jsonb_each(allowed_departure_modes) AS elem(key, value),
				jsonb_array_elements_text(elem.value) AS m(value)
			WHERE m.value = 'accompanied'
		);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed normalizing 'accompanied' in users.students.allowed_departure_modes: %w", err)
	}

	// Restore the narrower validators. With 'accompanied' already removed above,
	// no existing row violates them and future writes of that value are rejected.
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
