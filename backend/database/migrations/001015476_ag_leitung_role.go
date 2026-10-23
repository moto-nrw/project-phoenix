package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	agLeitungRoleVersion     = "1.15.476"
	agLeitungRoleDescription = "AG-Leitung system role with schedules:read (#4573, ADR 0090)"

	// agLeitungRoleName and agLeitungRoleText mirror
	// identityaccess.AGLeitungRoleName; migrations cannot import modules, so
	// keep both in sync by hand.
	agLeitungRoleName = "ag-leitung"
	agLeitungRoleText = "Betreut eigene Termine: starten, Kinder ein- und auschecken, beenden"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     agLeitungRoleVersion,
		Description: agLeitungRoleDescription,
		DependsOn:   []string{externalCaregiversTableVersion},
	})
	Migrations.MustRegister(agLeitungRoleUp, agLeitungRoleDown)
}

// agLeitungRoleUp adds the platform system role "ag-leitung" (ADR 0090): a
// person who runs their own blocks (start, check children in and out,
// complete) and nothing else. It is the access level „Eigene Termine
// betreuen“ of an external caregiver, but the role does not make anyone
// external; an internal colleague can hold it too.
//
// Deliberate choices, as for the lehrkraft role (1.15.278):
//   - A system role, not a per-school role: every school has it without
//     setup and no school can rename it away.
//   - base_role 'user', so the account carries a caregiver profile
//     (RoleNeedsCaregiverProfile) and the person stays plannable.
//   - Its only permission is schedules:read, which opens the own blocks of
//     the day. Its remaining reach is the follow-up of #4607.
//
// The insert guards with WHERE NOT EXISTS: system role names are unique only
// through a partial index (WHERE tenant_id IS NULL), which ON CONFLICT
// (name) cannot target.
func agLeitungRoleUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO auth.roles (name, description, is_system, base_role, tenant_id)
			SELECT ?, ?, TRUE, 'user', NULL
			WHERE NOT EXISTS (
				SELECT 1 FROM auth.roles WHERE name = ? AND tenant_id IS NULL
			)
		`, agLeitungRoleName, agLeitungRoleText, agLeitungRoleName); err != nil {
			return fmt.Errorf("error inserting ag-leitung role: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO auth.role_permissions (role_id, permission_id)
			SELECT r.id, p.id
			FROM auth.roles r
			CROSS JOIN auth.permissions p
			WHERE r.name = ? AND r.tenant_id IS NULL
			  AND p.name = 'schedules:read'
			ON CONFLICT (role_id, permission_id) DO NOTHING
		`, agLeitungRoleName); err != nil {
			return fmt.Errorf("error granting schedules:read to ag-leitung: %w", err)
		}
		return nil
	})
}

// agLeitungRoleDown removes the role with its grants and account
// assignments; open invitations into the role go with it.
func agLeitungRoleDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.invitation_tokens
			WHERE role_id IN (SELECT id FROM auth.roles WHERE name = 'ag-leitung' AND tenant_id IS NULL)
		`); err != nil {
			return fmt.Errorf("error removing ag-leitung invitations: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.account_roles
			WHERE role_id IN (SELECT id FROM auth.roles WHERE name = 'ag-leitung' AND tenant_id IS NULL)
		`); err != nil {
			return fmt.Errorf("error removing ag-leitung account roles: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.role_permissions
			WHERE role_id IN (SELECT id FROM auth.roles WHERE name = 'ag-leitung' AND tenant_id IS NULL)
		`); err != nil {
			return fmt.Errorf("error removing ag-leitung role permissions: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM auth.roles WHERE name = 'ag-leitung' AND tenant_id IS NULL`); err != nil {
			return fmt.Errorf("error removing ag-leitung role: %w", err)
		}
		return nil
	})
}
