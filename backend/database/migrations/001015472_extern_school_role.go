package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	externSchoolRoleVersion     = "1.15.472"
	externSchoolRoleDescription = "Provision the school role 'Extern' for every existing school that lacks it (#4573)"

	// externRoleName, externRoleDescription and externRolePermissions mirror
	// identityaccess.ExternalCaregiverRoleName (stored in lower case like
	// every custom role the role store writes),
	// identityaccess.ExternalCaregiverRoleDescription and
	// identityaccess.ExternalCaregiverDefaultPermissions, which school
	// provisioning uses for new schools. Migrations cannot import modules, so
	// keep the values in sync by hand.
	externRoleName        = "extern"
	externRoleDescription = "Für externe Betreuungskräfte. Kann den Betreuungsplan ansehen und eigene Termine betreuen."
)

var externRolePermissions = []string{"schedules:read"}

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     externSchoolRoleVersion,
		Description: externSchoolRoleDescription,
		DependsOn:   []string{"1.15.471"},
	})
	Migrations.MustRegister(externSchoolRoleUp, externSchoolRoleDown)
}

// externSchoolRoleUp gives every live school without a role named „Extern“
// (case-insensitive, trimmed) that role with its default permissions. A
// school that already has a role of that name keeps it untouched, whatever
// its tier or permissions, so a rerun is a no-op. Only the roles inserted
// here receive permissions.
func externSchoolRoleUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin extern school role: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			logRollbackFailure(ctx, err)
		}
	}()

	var available int
	if err := tx.NewRaw(`SELECT COUNT(*) FROM auth.permissions WHERE name IN (?)`, bun.List(externRolePermissions)).Scan(ctx, &available); err != nil {
		return fmt.Errorf("check extern role permissions: %w", err)
	}
	if available != len(externRolePermissions) {
		return fmt.Errorf("extern role needs the permissions %v, found %d of them", externRolePermissions, available)
	}

	var provisioned int
	err = tx.NewRaw(`
		WITH inserted AS (
			INSERT INTO auth.roles (tenant_id, name, description, is_system, base_role, created_at, updated_at)
			SELECT s.id, ?, ?, FALSE, 'user', NOW(), NOW()
			FROM platform.schools s
			WHERE s.deleted_at IS NULL
			  AND NOT EXISTS (
				SELECT 1 FROM auth.roles r
				WHERE r.tenant_id = s.id AND LOWER(TRIM(r.name)) = LOWER(TRIM(?))
			  )
			RETURNING id
		), granted AS (
			INSERT INTO auth.role_permissions (role_id, permission_id)
			SELECT i.id, p.id FROM inserted i CROSS JOIN auth.permissions p
			WHERE p.name IN (?)
			ON CONFLICT (role_id, permission_id) DO NOTHING
		)
		SELECT COUNT(*) FROM inserted`,
		externRoleName, externRoleDescription, externRoleName, bun.List(externRolePermissions)).Scan(ctx, &provisioned)
	if err != nil {
		return fmt.Errorf("provision extern school role: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit extern school role: %w", err)
	}
	migrationLog().InfoContext(ctx, "extern school role provisioned",
		"tenants", provisioned,
	)
	return nil
}

// externSchoolRoleDown deliberately keeps the data. Nothing marks a role as
// inserted here rather than created by the school, and a school may have
// edited or assigned it since, so deleting by name would destroy customer
// data. An extra role after a binary rollback is harmless.
func externSchoolRoleDown(_ context.Context, _ *bun.DB) error {
	return nil
}
