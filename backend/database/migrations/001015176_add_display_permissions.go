package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addDisplayPermissionsVersion     = "1.15.176"
	addDisplayPermissionsDescription = "Add display:read and display:manage permissions for info-point display management"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addDisplayPermissionsVersion,
		Description: addDisplayPermissionsDescription,
		DependsOn: []string{
			"1.5.3", // fix_permission_names — establishes the colon-separated naming scheme
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addDisplayPermissionsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addDisplayPermissionsDown(ctx, db)
		},
	)
}

func addDisplayPermissionsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.176: Adding display:read and display:manage permissions...")

	// No role grants: display management is admin territory and admins match
	// via the admin:* wildcard. The catalog rows exist so schools can grant
	// display:read to non-admin roles later without another migration.
	_, err := db.NewRaw(`
		INSERT INTO auth.permissions (name, description, resource, action, is_system)
		VALUES
			('display:read', 'View info-point displays and their status', 'display', 'read', TRUE),
			('display:manage', 'Create, rename, regenerate and delete info-point displays', 'display', 'manage', TRUE)
		ON CONFLICT (name) DO NOTHING;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed inserting display permissions: %w", err)
	}

	return nil
}

func addDisplayPermissionsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.176: Removing display permissions...")

	// role_permissions rows cascade on permission delete.
	_, err := db.NewRaw(`DELETE FROM auth.permissions WHERE name IN ('display:read', 'display:manage');`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed deleting display permissions: %w", err)
	}
	return nil
}
