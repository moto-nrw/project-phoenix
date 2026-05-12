package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addUsersCheckinPermissionVersion     = "1.15.49"
	addUsersCheckinPermissionDescription = "Add users:checkin permission for web-based student school check-in/out"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addUsersCheckinPermissionVersion,
		Description: addUsersCheckinPermissionDescription,
		DependsOn: []string{
			"1.5.3", // fix_permission_names — establishes the colon-separated naming scheme
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addUsersCheckinPermissionUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addUsersCheckinPermissionDown(ctx, db)
		},
	)
}

func addUsersCheckinPermissionUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.49: Adding users:checkin permission and granting to user role...")

	// Insert the permission. auth.permissions requires resource+action (see
	// 001000005_auth_permissions.go) and has a unique index on (resource, action)
	// alongside the name index — both conflicts map to the same "already exists"
	// case. Admin wildcards (admin:*) match any permission, so no admin grant
	// is needed; non-admin roles require an explicit grant below.
	_, err := db.NewRaw(`
		INSERT INTO auth.permissions (name, description, resource, action, is_system)
		VALUES ('users:checkin', 'Check students in and out of school via the web UI', 'users', 'checkin', TRUE)
		ON CONFLICT (name) DO NOTHING;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed inserting users:checkin permission: %w", err)
	}

	// Grant to the 'user' role (default staff role). Schools can further
	// restrict who uses it via the attendance.web_checkin_access setting
	// (group_supervisors vs all_staff) — this permission is the coarse gate,
	// the setting is the fine gate.
	_, err = db.NewRaw(`
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE r.name = 'user'
		AND p.name = 'users:checkin'
		ON CONFLICT (role_id, permission_id) DO NOTHING;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed granting users:checkin to user role: %w", err)
	}

	return nil
}

func addUsersCheckinPermissionDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.49: Removing users:checkin permission...")

	// role_permissions rows cascade on permission delete.
	_, err := db.NewRaw(`DELETE FROM auth.permissions WHERE name = 'users:checkin';`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed deleting users:checkin permission: %w", err)
	}
	return nil
}
