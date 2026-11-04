package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	schulverwaltungRoleVersion     = "1.15.520"
	schulverwaltungRoleDescription = "Schulverwaltung system role for Sekretariat and Schulleitung with class_day:all_classes (#4789)"

	// schulverwaltungRoleName mirrors identityaccess.SchulverwaltungRoleName;
	// migrations cannot import modules, so keep both in sync by hand.
	schulverwaltungRoleName = "schulverwaltung"
	schulverwaltungRoleText = "Sekretariat und Schulleitung: sieht in moto schule alle Klassen"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     schulverwaltungRoleVersion,
		Description: schulverwaltungRoleDescription,
		DependsOn: []string{
			lehrkraftRoleVersion,
			supervisionOwnPermissionVersion,
			schoolPortalArrivalExceptionsVersion,
			staffNoticesAudienceVersion,
			schoolPortalRolesVersion,
			// Lehrkraft permissions granted after #4574: the copy below takes
			// them only when it runs after their grant.
			morningAttendanceVersion,
			classHandoverVersion,
			schoolPortalClassTripsVersion,
		},
	})
	Migrations.MustRegister(schulverwaltungRoleUp, schulverwaltungRoleDown)
}

// schulverwaltungRoleUp adds the platform system role "schulverwaltung"
// (#4789): Sekretariat and Schulleitung in moto schule. Which of the two a
// person is shows only in their display name.
//
//   - A system role with school_portal = TRUE: every predicate that knows the
//     school-portal roles of #4574 (IsSchoolPortalRole, IsLehrkraftRole, the
//     push view, ClassifySchoolRoles) treats it like lehrkraft. An account
//     holding only this role signs in to moto schule and is refused by the
//     OGS portal.
//   - It starts with every permission lehrkraft holds, so its writes follow
//     operations.school_portal_write_scope like a Lehrkraft's.
//   - class_day:all_classes widens the class reach from the assigned classes
//     (education.class_teachers) to every class of the school, including
//     classes created later.
//
// The insert guards with WHERE NOT EXISTS: system role names are unique only
// through a partial index (WHERE tenant_id IS NULL), which ON CONFLICT
// (name) cannot target. Every grant names the system role through
// tenant_id IS NULL, so a school's own role of the same name stays untouched.
func schulverwaltungRoleUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO auth.permissions (name, description, resource, action)
			VALUES ('class_day:all_classes', 'Alle Klassen der Schule in moto schule sehen, ohne Zuweisung', 'class_day', 'all_classes')
			ON CONFLICT (name) DO NOTHING
		`); err != nil {
			return fmt.Errorf("error inserting class_day:all_classes: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO auth.roles (name, description, is_system, base_role, tenant_id, school_portal)
			SELECT ?, ?, TRUE, 'user', NULL, TRUE
			WHERE NOT EXISTS (
				SELECT 1 FROM auth.roles WHERE name = ? AND tenant_id IS NULL
			)
		`, schulverwaltungRoleName, schulverwaltungRoleText, schulverwaltungRoleName); err != nil {
			return fmt.Errorf("error inserting schulverwaltung role: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO auth.role_permissions (role_id, permission_id)
			SELECT target.id, granted.permission_id
			FROM auth.roles target
			CROSS JOIN (
				SELECT rp.permission_id
				FROM auth.role_permissions rp
				INNER JOIN auth.roles source ON source.id = rp.role_id
				WHERE source.name = 'lehrkraft' AND source.tenant_id IS NULL
				UNION
				-- The rights only the Schulverwaltung holds: its class reach and,
				-- once 1.15.683 created it, the report for a child missing
				-- without one (#4791). A rerun keeps them.
				SELECT p.id FROM auth.permissions p
				WHERE p.name IN ('class_day:all_classes', 'class_day:absence_report_write')
			) granted
			WHERE target.name = ? AND target.tenant_id IS NULL
			ON CONFLICT (role_id, permission_id) DO NOTHING
		`, schulverwaltungRoleName); err != nil {
			return fmt.Errorf("error granting permissions to schulverwaltung: %w", err)
		}
		return nil
	})
}

// schulverwaltungRoleDown removes the role with its grants and account
// assignments, then the permission; open invitations into the role go with
// it.
func schulverwaltungRoleDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		DELETE FROM auth.invitation_tokens
		WHERE role_id IN (SELECT id FROM auth.roles WHERE name = 'schulverwaltung' AND tenant_id IS NULL);
		DELETE FROM auth.account_roles
		WHERE role_id IN (SELECT id FROM auth.roles WHERE name = 'schulverwaltung' AND tenant_id IS NULL);
		DELETE FROM auth.role_permissions
		WHERE role_id IN (SELECT id FROM auth.roles WHERE name = 'schulverwaltung' AND tenant_id IS NULL);
		DELETE FROM auth.roles WHERE name = 'schulverwaltung' AND tenant_id IS NULL;
	`); err != nil {
		return fmt.Errorf("error removing schulverwaltung role: %w", err)
	}
	return dropPermission(ctx, db, "class_day:all_classes")
}
