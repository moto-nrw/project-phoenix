package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// permissionSpec describes one auth.permissions row.
type permissionSpec struct {
	Name        string
	Description string
	Resource    string
	Action      string
}

// grantPermissionToRoles registers a permission and grants it to the named
// roles, both idempotently, inside one transaction. Twenty-two migrations
// hand-rolled this same INSERT pair plus its transaction ceremony before this
// helper existed; new permission migrations should call it instead.
func grantPermissionToRoles(ctx context.Context, db *bun.DB, spec permissionSpec, roles ...string) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO auth.permissions (name, description, resource, action)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (name) DO NOTHING
		`, spec.Name, spec.Description, spec.Resource, spec.Action); err != nil {
			return fmt.Errorf("error inserting %s: %w", spec.Name, err)
		}

		if len(roles) == 0 {
			return nil
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO auth.role_permissions (role_id, permission_id)
			SELECT r.id, p.id
			FROM auth.roles r
			CROSS JOIN auth.permissions p
			WHERE p.name = ?
			  AND r.name IN (?)
			ON CONFLICT (role_id, permission_id) DO NOTHING
		`, spec.Name, bun.List(roles)); err != nil {
			return fmt.Errorf("error granting %s to %v: %w", spec.Name, roles, err)
		}

		return nil
	})
}

// dropPermission removes a permission and every role grant of it — the inverse
// of grantPermissionToRoles, for migration rollbacks.
func dropPermission(ctx context.Context, db *bun.DB, name string) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.role_permissions
			WHERE permission_id IN (
				SELECT id FROM auth.permissions WHERE name = ?
			)
		`, name); err != nil {
			return fmt.Errorf("error removing role permissions for %s: %w", name, err)
		}

		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.permissions WHERE name = ?
		`, name); err != nil {
			return fmt.Errorf("error removing permission %s: %w", name, err)
		}

		return nil
	})
}
