package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	lehrkraftRoleVersion     = "1.15.278"
	lehrkraftRoleDescription = "Lehrkraft system role with class_day:read permission (#1772)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     lehrkraftRoleVersion,
		Description: lehrkraftRoleDescription,
		DependsOn:   []string{classTeachersVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return lehrkraftRoleUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return lehrkraftRoleDown(ctx, db)
		},
	)
}

// lehrkraftRoleUp adds the platform system role "lehrkraft" (#1772): a school
// teacher who gets a read-only per-class day view and nothing else.
//
// Deliberate choices:
//   - A system role, not a per-tenant custom role: every school gets it
//     without setup, and the school-access flows create the users.staff record
//     for it automatically (EnsureSchoolIdentity). No users.teachers caregiver
//     profile is ever created — RoleNeedsCaregiverProfile excludes this role
//     explicitly, and the caregiver_enabled upgrade is refused for it
//     (IsLehrkraftSystemRole guards invitation creation and both acceptance
//     branches).
//   - base_role 'user' (the CHECK allows admin/user/guardian only); the name
//     differs from the retired "teacher" role, which the assignment policy
//     blocks by name.
//   - Its only permission is class_day:read. No users:read: the generic
//     student directory routes stay closed, scoping happens exclusively via
//     education.class_teachers.
//
// The insert guards with WHERE NOT EXISTS instead of ON CONFLICT: since
// migration 1.14.3 the uniqueness of system role names is a partial index
// (WHERE tenant_id IS NULL), which a bare ON CONFLICT (name) cannot target.
func lehrkraftRoleUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.278: Lehrkraft system role with class_day:read...")

	if _, err := db.ExecContext(ctx, `
		INSERT INTO auth.roles (name, description, is_system, base_role, tenant_id)
		SELECT 'lehrkraft',
		       'Lehrkraft mit Lesezugriff auf die Tagesansicht der zugewiesenen Klassen',
		       TRUE,
		       'user',
		       NULL
		WHERE NOT EXISTS (
			SELECT 1 FROM auth.roles WHERE name = 'lehrkraft' AND tenant_id IS NULL
		)
	`); err != nil {
		return fmt.Errorf("error inserting lehrkraft role: %w", err)
	}

	return grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        "class_day:read",
		Description: "Tagesansicht der zugewiesenen Klassen einsehen",
		Resource:    "class_day",
		Action:      "read",
	}, "admin", "lehrkraft")
}

func lehrkraftRoleDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.278: Removing lehrkraft role and class_day:read...")

	if err := dropPermission(ctx, db, "class_day:read"); err != nil {
		return err
	}

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.account_roles
			WHERE role_id IN (
				SELECT id FROM auth.roles WHERE name = 'lehrkraft' AND tenant_id IS NULL
			)
		`); err != nil {
			return fmt.Errorf("error removing lehrkraft account roles: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.role_permissions
			WHERE role_id IN (
				SELECT id FROM auth.roles WHERE name = 'lehrkraft' AND tenant_id IS NULL
			)
		`); err != nil {
			return fmt.Errorf("error removing lehrkraft role permissions: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.roles WHERE name = 'lehrkraft' AND tenant_id IS NULL
		`); err != nil {
			return fmt.Errorf("error removing lehrkraft role: %w", err)
		}

		return nil
	})
}
