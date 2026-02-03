package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	enumOptionsVersion     = "1.8.3"
	enumOptionsDescription = "Add enum_options JSONB column to setting_definitions"
)

// EnumOptionsDependsOn defines migration dependencies
var EnumOptionsDependsOn = []string{
	hierarchicalSettingsVersion, // Depends on hierarchical settings (1.8.2)
}

func init() {
	MigrationRegistry[enumOptionsVersion] = &Migration{
		Version:     enumOptionsVersion,
		Description: enumOptionsDescription,
		DependsOn:   EnumOptionsDependsOn,
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return enumOptionsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return enumOptionsDown(ctx, db)
		},
	)
}

func enumOptionsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.3: Adding enum_options column to setting_definitions...")

	// Add enum_options column (JSONB array of {value, label} objects)
	_, err := db.NewRaw(`
		ALTER TABLE config.setting_definitions
		ADD COLUMN IF NOT EXISTS enum_options JSONB;

		-- Add a comment explaining the column
		COMMENT ON COLUMN config.setting_definitions.enum_options IS
			'Array of {value, label} objects for enum type settings with display labels';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding enum_options column: %w", err)
	}

	fmt.Println("Migration 1.8.3: enum_options column added successfully")
	return nil
}

func enumOptionsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.3: Removing enum_options column from setting_definitions...")

	_, err := db.NewRaw(`
		ALTER TABLE config.setting_definitions
		DROP COLUMN IF EXISTS enum_options;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed removing enum_options column: %w", err)
	}

	fmt.Println("Migration 1.8.3: enum_options column removed successfully")
	return nil
}
