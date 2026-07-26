package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addIsSystemFlagsVersion     = "1.15.168"
	addIsSystemFlagsDescription = "Add is_system flag to facilities.rooms, activities.groups and activities.categories so auto-provisioned Schulhof/WC infrastructure can be excluded from staff-facing lists (issue #923)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addIsSystemFlagsVersion,
		Description: addIsSystemFlagsDescription,
		DependsOn: []string{
			phaseCalendarPeriodVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.168: Adding is_system flags to rooms, activity groups and categories...")
			if _, err := db.NewRaw(`
				ALTER TABLE facilities.rooms
				ADD COLUMN IF NOT EXISTS is_system BOOLEAN NOT NULL DEFAULT FALSE;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding is_system column to facilities.rooms: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE activities.groups
				ADD COLUMN IF NOT EXISTS is_system BOOLEAN NOT NULL DEFAULT FALSE;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding is_system column to activities.groups: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE activities.categories
				ADD COLUMN IF NOT EXISTS is_system BOOLEAN NOT NULL DEFAULT FALSE;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding is_system column to activities.categories: %w", err)
			}

			// Backfill existing auto-provisioned system rows.
			// Rooms and categories have UNIQUE(tenant_id, name), so flagging by
			// name is unambiguous. Activity group names are NOT unique — a staff
			// member can legitimately own an activity named "WC" — so groups are
			// additionally restricted to created_by IS NULL, which only the
			// system provisioning code paths produce.
			if _, err := db.NewRaw(`
				UPDATE facilities.rooms
				SET is_system = TRUE
				WHERE name IN ('Schulhof', 'WC', 'Toilette');
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed flagging system rooms: %w", err)
			}
			if _, err := db.NewRaw(`
				UPDATE activities.categories
				SET is_system = TRUE
				WHERE name IN ('Schulhof', 'WC');
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed flagging system categories: %w", err)
			}
			if _, err := db.NewRaw(`
				UPDATE activities.groups
				SET is_system = TRUE
				WHERE name IN ('Schulhof Freispiel', 'WC')
				  AND created_by IS NULL;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed flagging system activity groups: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.168...")
			if _, err := db.NewRaw(`
				ALTER TABLE activities.categories
				DROP COLUMN IF EXISTS is_system;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping is_system column from activities.categories: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE activities.groups
				DROP COLUMN IF EXISTS is_system;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping is_system column from activities.groups: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE facilities.rooms
				DROP COLUMN IF EXISTS is_system;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping is_system column from facilities.rooms: %w", err)
			}
			return nil
		},
	)
}
