package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	mealPermissionsVersion     = "1.15.481"
	mealPermissionsDescription = "Mittagessen: eigene Rechte meals:read, meals:manage und meals:orders statt Konfiguration und Benutzerdaten (#3873)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mealPermissionsVersion,
		Description: mealPermissionsDescription,
		DependsOn:   []string{mealAbsenceHistoryVersion},
	})
	Migrations.MustRegister(mealPermissionsUp, mealPermissionsDown)
}

// mealPermissionsUp gives the meal plan permissions of its own (#3873). Until
// now the meal plan read config:read, its changes config:update, and the
// kitchen list config:read together with users:read, so a kitchen role
// needed the whole configuration and every child's data.
//
// Every role and every directly granted account keeps exactly what it had:
// meals:read where config:read was, meals:manage where config:update was,
// and meals:orders where both config:read and users:read were. The old
// check of the kitchen list took both from everything an account held at a
// school, so an account whose two permissions come from different roles or
// direct grants gets meals:orders as a direct grant of that school. Admins
// hold all of them through admin:* anyway.
func mealPermissionsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if err := renameMealLeadDays(ctx, tx, "operations.meal_registration_cutoff_lead_days", "operations.meal_registration_lead_days"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO auth.permissions (name, description, resource, action, is_system) VALUES
				('meals:read', 'View the meal plan and the number of meals ordered', 'meals', 'read', TRUE),
				('meals:manage', 'Maintain the meal plan and the school''s caterer', 'meals', 'manage', TRUE),
				('meals:orders', 'See meal lists with children''s names and change a registration after the deadline', 'meals', 'orders', TRUE)
			ON CONFLICT (name) DO NOTHING;
		`); err != nil {
			return fmt.Errorf("insert meal permissions: %w", err)
		}
		if _, err := tx.ExecContext(ctx, grantMealPermissionsSQL); err != nil {
			return fmt.Errorf("grant meal permissions where the old ones were: %w", err)
		}
		return nil
	})
}

func mealPermissionsDown(ctx context.Context, db *bun.DB) error {
	for _, name := range []string{"meals:read", "meals:manage", "meals:orders"} {
		if _, err := db.ExecContext(ctx, `DELETE FROM auth.account_permissions WHERE permission_id IN (SELECT id FROM auth.permissions WHERE name = ?)`, name); err != nil {
			return fmt.Errorf("remove account grants of %s: %w", name, err)
		}
		if err := dropPermission(ctx, db, name); err != nil {
			return err
		}
	}
	return renameMealLeadDays(ctx, db, "operations.meal_registration_lead_days", "operations.meal_registration_cutoff_lead_days")
}

// Keep stored lead counts when moving to the ordering rules. An explicit
// value at the destination wins, including zero; a rerun changes nothing.
func renameMealLeadDays(ctx context.Context, db bun.IDB, from, to string) error {
	_, err := db.NewRaw(`
		DELETE FROM config.setting_values AS old
		USING config.setting_values AS current
		WHERE old.tenant_id = current.tenant_id AND old.setting_key = ? AND current.setting_key = ?;
		UPDATE config.setting_values SET setting_key = ? WHERE setting_key = ?;
	`, from, to, to, from).Exec(ctx)
	if err != nil {
		return fmt.Errorf("move meal registration lead days: %w", err)
	}
	return nil
}

const grantMealPermissionsSQL = `
			WITH grants (meal, required) AS (
				VALUES ('meals:read', ARRAY['config:read']),
					('meals:manage', ARRAY['config:update']),
					('meals:orders', ARRAY['config:read', 'users:read'])
			)
			INSERT INTO auth.role_permissions (role_id, permission_id)
			SELECT role_permission.role_id, meal_permission.id
			FROM grants
			JOIN auth.permissions AS meal_permission ON meal_permission.name = grants.meal
			JOIN auth.role_permissions AS role_permission ON TRUE
			JOIN auth.permissions AS held ON held.id = role_permission.permission_id
			GROUP BY role_permission.role_id, meal_permission.id, grants.required
			HAVING array_agg(held.name::text) @> grants.required::text[]
			ON CONFLICT (role_id, permission_id) DO NOTHING;

			WITH grants (meal, required) AS (
				VALUES ('meals:read', ARRAY['config:read']),
					('meals:manage', ARRAY['config:update']),
					('meals:orders', ARRAY['config:read', 'users:read'])
			)
			INSERT INTO auth.account_permissions (tenant_id, account_id, permission_id, granted)
			SELECT account_permission.tenant_id, account_permission.account_id, meal_permission.id, TRUE
			FROM grants
			JOIN auth.permissions AS meal_permission ON meal_permission.name = grants.meal
			JOIN auth.account_permissions AS account_permission ON account_permission.granted
			JOIN auth.permissions AS held ON held.id = account_permission.permission_id
			GROUP BY account_permission.tenant_id, account_permission.account_id, meal_permission.id, grants.required
			HAVING array_agg(held.name::text) @> grants.required::text[]
			ON CONFLICT DO NOTHING;

			WITH held (account_id, tenant_id, name) AS (
				SELECT account_role.account_id, account_role.tenant_id, permission.name::text
				FROM auth.account_roles AS account_role
				JOIN auth.role_permissions AS role_permission ON role_permission.role_id = account_role.role_id
				JOIN auth.permissions AS permission ON permission.id = role_permission.permission_id
				UNION
				SELECT account_permission.account_id, account_permission.tenant_id, permission.name::text
				FROM auth.account_permissions AS account_permission
				JOIN auth.permissions AS permission ON permission.id = account_permission.permission_id
				WHERE account_permission.granted
			)
			INSERT INTO auth.account_permissions (tenant_id, account_id, permission_id, granted)
			SELECT held.tenant_id, held.account_id, meal_permission.id, TRUE
			FROM held
			JOIN auth.permissions AS meal_permission ON meal_permission.name = 'meals:orders'
			GROUP BY held.tenant_id, held.account_id, meal_permission.id
			HAVING array_agg(held.name) @> ARRAY['config:read', 'users:read']
				AND NOT bool_or(held.name = 'meals:orders')
			ON CONFLICT DO NOTHING;
		`
