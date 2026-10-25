package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const mealCombinedRolePermissionsVersion = "1.15.549"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mealCombinedRolePermissionsVersion,
		Description: "Essenslisten: Zugriff aus getrennten Rollen als Rollenrecht erhalten (#3873)",
		DependsOn:   []string{mealPermissionsVersion},
	})
	Migrations.MustRegister(mealCombinedRolePermissionsUp, mealCombinedRolePermissionsDown)
}

// Direct account grants are retired (#4485). Preserve the old conjunction
// only for accounts whose roles still provide both required permissions in
// the same school. Neither original role gains a permission for its other
// members. Keep school-only accounts school-only; the supplemental roles
// have no base role and confer only meals:orders, never portal/admin rights.
const mealCombinedRoleAccountsSQL = `
	WITH eligible AS (
		SELECT ar.account_id, ar.tenant_id,
			bool_and(r.school_portal OR (r.is_system AND lower(btrim(r.name)) = 'lehrkraft')) AS school_portal
		FROM auth.account_roles ar
		JOIN auth.roles r ON r.id = ar.role_id
		LEFT JOIN auth.role_permissions rp ON rp.role_id = ar.role_id
		LEFT JOIN auth.permissions p ON p.id = rp.permission_id
		GROUP BY ar.account_id, ar.tenant_id
		HAVING array_agg(p.name::text) @> ARRAY['config:read', 'users:read']
			AND NOT bool_or(p.name IN ('meals:orders', 'admin:*'))
	)
`

func mealCombinedRolePermissionsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, mealCombinedRoleAccountsSQL+`
			INSERT INTO auth.roles (name, description, is_system, base_role, tenant_id, school_portal)
			SELECT CASE WHEN school_portal THEN 'meal-orders-migrated-school' ELSE 'meal-orders-migrated' END,
				'Essenslisten ansehen und Anmeldungen nach der Frist ändern.', TRUE, NULL, NULL, school_portal
			FROM (SELECT DISTINCT school_portal FROM eligible) needed
			WHERE NOT EXISTS (
				SELECT 1 FROM auth.roles r WHERE r.tenant_id IS NULL
				AND r.name = CASE WHEN needed.school_portal THEN 'meal-orders-migrated-school' ELSE 'meal-orders-migrated' END
			);
			DO $$ BEGIN
				IF EXISTS (
					SELECT 1 FROM auth.roles r WHERE r.tenant_id IS NULL
					AND r.name IN ('meal-orders-migrated', 'meal-orders-migrated-school')
					AND (NOT r.is_system OR r.base_role IS NOT NULL
						OR r.school_portal <> (r.name = 'meal-orders-migrated-school')
						OR EXISTS (SELECT 1 FROM auth.role_permissions rp JOIN auth.permissions p ON p.id = rp.permission_id
							WHERE rp.role_id = r.id AND p.name <> 'meals:orders'))
				) THEN
					RAISE EXCEPTION 'meal migration role has incompatible permissions or portal scope';
				END IF;
			END $$;
			INSERT INTO auth.role_permissions (role_id, permission_id)
			SELECT r.id, p.id FROM auth.roles r CROSS JOIN auth.permissions p
			WHERE r.tenant_id IS NULL AND r.name IN ('meal-orders-migrated', 'meal-orders-migrated-school')
			AND p.name = 'meals:orders'
			ON CONFLICT (role_id, permission_id) DO NOTHING;
		`); err != nil {
			return fmt.Errorf("prepare combined meal permission roles: %w", err)
		}
		if _, err := tx.ExecContext(ctx, mealCombinedRoleAccountsSQL+`
			INSERT INTO auth.account_roles (tenant_id, account_id, role_id)
			SELECT e.tenant_id, e.account_id, r.id FROM eligible e
			JOIN auth.roles r ON r.tenant_id IS NULL
				AND r.name = CASE WHEN e.school_portal THEN 'meal-orders-migrated-school' ELSE 'meal-orders-migrated' END
			ON CONFLICT DO NOTHING;
		`); err != nil {
			return fmt.Errorf("preserve combined meal permissions through roles: %w", err)
		}
		return nil
	})
}

func mealCombinedRolePermissionsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		DELETE FROM auth.invitation_tokens WHERE role_id IN (
			SELECT id FROM auth.roles WHERE tenant_id IS NULL AND name IN ('meal-orders-migrated', 'meal-orders-migrated-school'));
		DELETE FROM auth.account_roles WHERE role_id IN (
			SELECT id FROM auth.roles WHERE tenant_id IS NULL AND name IN ('meal-orders-migrated', 'meal-orders-migrated-school'));
		DELETE FROM auth.role_permissions WHERE role_id IN (
			SELECT id FROM auth.roles WHERE tenant_id IS NULL AND name IN ('meal-orders-migrated', 'meal-orders-migrated-school'));
		DELETE FROM auth.roles WHERE tenant_id IS NULL AND name IN ('meal-orders-migrated', 'meal-orders-migrated-school');
	`); err != nil {
		return fmt.Errorf("remove combined meal permission roles: %w", err)
	}
	return nil
}
