package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	guestViewOnlyVersion     = "1.15.479"
	guestViewOnlyDescription = "Guest role only views: students:read, groups:read, rooms:read, activities:read (#4573, ADR 0091)"

	guestViewOnlyText = "Nur ansehen: Kinder, Räume und AGs. Keine Elterndaten, keine Nachrichten."
	// guestPreviousText is the description migration 1.0.4 gave the role.
	guestPreviousText = "Limited access for unauthenticated users"
)

// guestViewOnlyPermissions are the permissions the system role guest holds
// after this migration, and guestPreviousPermissions the ones Down restores
// (migration 1.7.0).
var (
	guestViewOnlyPermissions = []string{"students:read", "groups:read", "rooms:read", "activities:read"}
	guestPreviousPermissions = []string{"users:read", "users:list", "rooms:read", "activities:read", "activities:create"}
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guestViewOnlyVersion,
		Description: guestViewOnlyDescription,
		DependsOn:   []string{GuestPermissionsVersion, agLeitungRoleVersion},
	})
	Migrations.MustRegister(guestViewOnlyUp, guestViewOnlyDown)
}

// guestViewOnlyUp makes the system role guest („Gast“) the access level
// „Nur ansehen“ (ADR 0091). users:read opened, besides the child list and
// detail, the guardian directory, parent messages, student notes and the
// exports; users:list and activities:create were never needed for looking.
// The new permission students:read opens only the read routes of the child
// list and detail, so a guest sees children, rooms and AGs but no parent
// data and no messages. No other role receives students:read: the admin
// holds *:* and every staff role keeps users:read, which opens the same
// routes.
func guestViewOnlyUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO auth.permissions (name, description, resource, action)
			VALUES ('students:read', 'Kinder ansehen, ohne Elterndaten', 'students', 'read')
			ON CONFLICT (name) DO NOTHING
		`); err != nil {
			return fmt.Errorf("error inserting students:read: %w", err)
		}
		return setGuestRole(ctx, tx, guestViewOnlyText, guestViewOnlyPermissions)
	})
}

// guestViewOnlyDown gives the role its previous permissions and description
// back and removes students:read with every grant of it.
func guestViewOnlyDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := setGuestRole(ctx, tx, guestPreviousText, guestPreviousPermissions); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM auth.role_permissions
			WHERE permission_id IN (SELECT id FROM auth.permissions WHERE name = 'students:read')
		`); err != nil {
			return fmt.Errorf("error removing students:read grants: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM auth.permissions WHERE name = 'students:read'`); err != nil {
			return fmt.Errorf("error removing students:read: %w", err)
		}
		return nil
	})
}

// setGuestRole sets the system role guest to exactly the permissions and the
// description. A school's own role named guest stays untouched.
func setGuestRole(ctx context.Context, tx bun.Tx, description string, permissions []string) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE auth.roles SET description = ?, updated_at = NOW()
		WHERE name = 'guest' AND tenant_id IS NULL
	`, description); err != nil {
		return fmt.Errorf("error describing guest role: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE role_id IN (SELECT id FROM auth.roles WHERE name = 'guest' AND tenant_id IS NULL)
		  AND permission_id NOT IN (SELECT id FROM auth.permissions WHERE name IN (?))
	`, bun.List(permissions)); err != nil {
		return fmt.Errorf("error narrowing guest permissions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE r.name = 'guest' AND r.tenant_id IS NULL
		  AND p.name IN (?)
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`, bun.List(permissions)); err != nil {
		return fmt.Errorf("error granting guest permissions: %w", err)
	}
	return nil
}
