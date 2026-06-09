package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	formSchemasPickupWeekdayBooleanVersion     = "1.15.117"
	formSchemasPickupWeekdayBooleanDescription = "Convert existing pickup_status form fields from select to weekday_boolean"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     formSchemasPickupWeekdayBooleanVersion,
		Description: formSchemasPickupWeekdayBooleanDescription,
		DependsOn: []string{
			createEnrollmentFormSchemasVersion,
			studentsPickupDaysVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return formSchemasPickupWeekdayBooleanUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			// Down is intentionally a no-op: we cannot reconstruct the
			// admin's original select options, and the weekday_boolean shape
			// is forward-compatible. Rolling back the column migration
			// (1.15.116) does not require reverting persisted form schemas.
			return nil
		},
	)
}

// formSchemasPickupWeekdayBooleanUp rewrites every persisted form field whose
// target is student.pickup_status so its type becomes weekday_boolean and any
// leftover select `options` are dropped. ReservedTargets now declares
// pickup_status as weekday_boolean, and FormField.Validate() rejects a
// targeted field whose stored type no longer matches the reserved type — so
// without this rewrite, any school that already added the "Abholregelung"
// field to its enrollment form would fail validation on the next load/save.
func formSchemasPickupWeekdayBooleanUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.117: Converting pickup_status form fields to weekday_boolean...")

	if _, err := db.NewRaw(`
		UPDATE enrollment.form_schemas
		SET fields = (
			SELECT jsonb_agg(
				CASE
					WHEN elem->>'target' = 'student.pickup_status'
					THEN (elem - 'options') || '{"type":"weekday_boolean"}'::jsonb
					ELSE elem
				END
			)
			FROM jsonb_array_elements(fields) AS elem
		)
		WHERE fields @> '[{"target":"student.pickup_status"}]'::jsonb;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed converting pickup_status form fields: %w", err)
	}

	return nil
}
