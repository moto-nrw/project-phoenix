package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addUsersAbsencePermissionVersion     = "1.15.295"
	addUsersAbsencePermissionDescription = "Add users:absence permission for managing student absence statuses without fixed groups"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addUsersAbsencePermissionVersion,
		Description: addUsersAbsencePermissionDescription,
		DependsOn: []string{
			"1.5.3", // fix_permission_names — establishes the colon-separated naming scheme
			"1.9.4", // consolidate_roles — 'user' is the default staff (Betreuer) role
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addUsersAbsencePermissionUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addUsersAbsencePermissionDown(ctx, db)
		},
	)
}

func addUsersAbsencePermissionUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.295: Adding users:absence permission and granting to user role...")

	// Mirrors 1.15.49 (users:checkin): auth.permissions requires resource+action
	// and is unique on both (resource, action) and name. Admin wildcards
	// (admin:*) match any permission, so admins need no grant.
	_, err := db.NewRaw(`
		INSERT INTO auth.permissions (name, description, resource, action, is_system)
		VALUES ('users:absence', 'Report and clear student absences (sick, excused, class trip)', 'users', 'absence', TRUE)
		ON CONFLICT (name) DO NOTHING;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed inserting users:absence permission: %w", err)
	}

	// Grant to the 'user' role (default staff/Betreuer role). It only widens
	// anything in a tenant running operations.group_mode = open_care, where
	// there are no groups to supervise; under fixed_groups the supervisor gate
	// still decides every absence write.
	_, err = db.NewRaw(`
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE r.name = 'user'
		AND p.name = 'users:absence'
		ON CONFLICT (role_id, permission_id) DO NOTHING;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed granting users:absence to user role: %w", err)
	}

	return nil
}

func addUsersAbsencePermissionDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.295: Removing users:absence permission...")

	// role_permissions rows cascade on permission delete.
	_, err := db.NewRaw(`DELETE FROM auth.permissions WHERE name = 'users:absence';`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed deleting users:absence permission: %w", err)
	}
	return nil
}
