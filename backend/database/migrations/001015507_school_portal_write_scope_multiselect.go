package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	schoolPortalWriteScopeMultiSelectVersion = "1.15.507"
	schoolPortalWriteScopeMultiSelectKey     = "operations.school_portal_write_scope"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     schoolPortalWriteScopeMultiSelectVersion,
		Description: "Turn the school portal write scope into a multi-select and keep each school's choice (#4799)",
		DependsOn: []string{
			configSettingValuesVersion, // config.setting_values
			moduleNFCSwitchVersion,     // latest migration at authoring time
		},
	})
	Migrations.MustRegister(schoolPortalWriteScopeMultiSelectUp, schoolPortalWriteScopeMultiSelectDown)
}

// schoolPortalWriteScopeMultiSelectUp rewrites the single-select rows as
// lists: "class_arrival_exceptions" keeps that write open, "none" (and any
// other string, which validation never accepted) becomes the empty list. Rows
// that already hold a list stay, so a second run changes nothing. The audit
// trail keeps the values as they were written.
func schoolPortalWriteScopeMultiSelectUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		UPDATE config.setting_values
		SET value = CASE
				WHEN value = '"class_arrival_exceptions"'::jsonb THEN '["class_arrival_exceptions"]'::jsonb
				ELSE '[]'::jsonb
			END,
			updated_at = now()
		WHERE setting_key = ?
			AND jsonb_typeof(value) = 'string'
	`, schoolPortalWriteScopeMultiSelectKey).Exec(ctx)
	if err != nil {
		return fmt.Errorf("convert school portal write scope to a list: %w", err)
	}
	return nil
}

// schoolPortalWriteScopeMultiSelectDown restores the single-select: a list
// with the class arrival exceptions becomes that value, every other list
// "none". A chosen class trip write has no single-select value and closes.
func schoolPortalWriteScopeMultiSelectDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		UPDATE config.setting_values
		SET value = CASE
				WHEN value @> '["class_arrival_exceptions"]'::jsonb THEN '"class_arrival_exceptions"'::jsonb
				ELSE '"none"'::jsonb
			END,
			updated_at = now()
		WHERE setting_key = ?
			AND jsonb_typeof(value) = 'array'
	`, schoolPortalWriteScopeMultiSelectKey).Exec(ctx)
	if err != nil {
		return fmt.Errorf("convert school portal write scope back to a single value: %w", err)
	}
	return nil
}
