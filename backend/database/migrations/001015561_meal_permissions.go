package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	mealPermissionsVersion     = "1.15.561"
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
		if err := moveMealLeadDays(ctx, tx, "operations.meal_registration_cutoff_lead_days", "operations.meal_registration_lead_days", true); err != nil {
			return err
		}
		if err := adoptMealCutoffSettings(ctx, tx); err != nil {
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
	if err := restoreMealCutoffSettings(ctx, db); err != nil {
		return err
	}
	return moveMealLeadDays(ctx, db, "operations.meal_registration_lead_days", "operations.meal_registration_cutoff_lead_days", false)
}

// adoptMealCutoffSettings carries the cutoffs of #3164 into the ordering
// rules: a weekday of the previous week becomes a registration deadline of
// that kind, and a separate cancellation cutoff the model with its own
// cancellation deadline, keeping its lead days. The cancellation clock time
// already has its key. An explicit value of the ordering rules wins; a rerun
// changes nothing.
func adoptMealCutoffSettings(ctx context.Context, db bun.IDB) error {
	if _, err := db.ExecContext(ctx, adoptMealCutoffSettingsSQL); err != nil {
		return fmt.Errorf("adopt the meal cutoff settings: %w", err)
	}
	return moveMealLeadDays(ctx, db, "operations.meal_cancellation_cutoff_lead_days", "operations.meal_cancellation_lead_days", true)
}

func restoreMealCutoffSettings(ctx context.Context, db bun.IDB) error {
	if _, err := db.ExecContext(ctx, restoreMealCutoffSettingsSQL); err != nil {
		return fmt.Errorf("restore the meal cutoff settings: %w", err)
	}
	return moveMealLeadDays(ctx, db, "operations.meal_cancellation_lead_days", "operations.meal_cancellation_cutoff_lead_days", false)
}

const adoptMealCutoffSettingsSQL = `
	INSERT INTO config.setting_values (tenant_id, setting_key, value)
	SELECT tenant_id, key, value FROM (
		SELECT tenant_id, 'operations.meal_registration_deadline_kind' AS key, to_jsonb('previous_week'::text) AS value
		FROM config.setting_values
		WHERE setting_key = 'operations.meal_registration_cutoff_weekday' AND value #>> '{}' IN ('1', '2', '3', '4', '5')
		UNION ALL
		SELECT tenant_id, 'operations.meal_registration_deadline_weekday',
			to_jsonb((ARRAY['monday', 'tuesday', 'wednesday', 'thursday', 'friday'])[(value #>> '{}')::int])
		FROM config.setting_values
		WHERE setting_key = 'operations.meal_registration_cutoff_weekday' AND value #>> '{}' IN ('1', '2', '3', '4', '5')
		UNION ALL
		SELECT tenant_id, 'operations.meal_registration_model', to_jsonb('ongoing_split'::text)
		FROM config.setting_values
		WHERE setting_key = 'operations.meal_cancellation_cutoff_separate' AND value = 'true'::jsonb
		UNION ALL
		SELECT tenant_id, 'operations.meal_cancellation_deadline_kind', to_jsonb('days_before'::text)
		FROM config.setting_values
		WHERE setting_key = 'operations.meal_cancellation_cutoff_separate' AND value = 'true'::jsonb
	) AS adopted
	ON CONFLICT (tenant_id, setting_key) DO NOTHING;
	DELETE FROM config.setting_values
	WHERE setting_key IN ('operations.meal_registration_cutoff_weekday', 'operations.meal_cancellation_cutoff_separate');
`

const restoreMealCutoffSettingsSQL = `
	INSERT INTO config.setting_values (tenant_id, setting_key, value)
	SELECT kind.tenant_id, 'operations.meal_registration_cutoff_weekday',
		to_jsonb(array_position(ARRAY['monday', 'tuesday', 'wednesday', 'thursday', 'friday'], weekday.value #>> '{}'))
	FROM config.setting_values AS kind
	JOIN config.setting_values AS weekday ON weekday.tenant_id = kind.tenant_id
		AND weekday.setting_key = 'operations.meal_registration_deadline_weekday'
	WHERE kind.setting_key = 'operations.meal_registration_deadline_kind' AND kind.value = '"previous_week"'::jsonb
		AND weekday.value #>> '{}' IN ('monday', 'tuesday', 'wednesday', 'thursday', 'friday')
	ON CONFLICT (tenant_id, setting_key) DO NOTHING;
	INSERT INTO config.setting_values (tenant_id, setting_key, value)
	SELECT tenant_id, 'operations.meal_cancellation_cutoff_separate', 'true'::jsonb
	FROM config.setting_values
	WHERE setting_key = 'operations.meal_registration_model' AND value = '"ongoing_split"'::jsonb
	ON CONFLICT (tenant_id, setting_key) DO NOTHING;
`

// Development's #4866 counts lead days as working days: weekends, holidays
// and closing days do not count. The ordering rules count calendar days. N
// working days span at most N + 2*ceil(N/5) calendar days across weekends
// (1 -> 3, 2 -> 4, 5 -> 7, 6 -> 10), so Up stores that span and no deadline
// moves later unless a holiday or closing day falls into the lead
// (docs/operations/essensfristen-umrechnung-3873.md lists those schools).
// 0 stays 0. The conversion is strictly increasing, so Down maps every value
// Up wrote back to exactly its N; a value a school set afterwards that no N
// converts to becomes the smallest N whose span covers it, so the deadline
// does not move later across weekends either.
const (
	workingToCalendarLeadDaysSQL = `
		DELETE FROM config.setting_values AS old
		USING config.setting_values AS current
		WHERE old.tenant_id = current.tenant_id AND old.setting_key = ? AND current.setting_key = ?;
		UPDATE config.setting_values SET setting_key = ?,
			value = CASE WHEN jsonb_typeof(value) = 'number' AND (value::text)::numeric >= 0
				THEN to_jsonb(((value::text)::numeric::int + 2 * ceil((value::text)::numeric::int / 5.0))::int)
				ELSE value END
		WHERE setting_key = ?;
	`
	calendarToWorkingLeadDaysSQL = `
		DELETE FROM config.setting_values AS old
		USING config.setting_values AS current
		WHERE old.tenant_id = current.tenant_id AND old.setting_key = ? AND current.setting_key = ?;
		UPDATE config.setting_values SET setting_key = ?,
			value = CASE WHEN jsonb_typeof(value) = 'number' AND (value::text)::numeric >= 0
				THEN to_jsonb((SELECT min(n) FROM generate_series(0, (value::text)::numeric::int) AS n
					WHERE n + 2 * ceil(n / 5.0) >= (value::text)::numeric::int)::int)
				ELSE value END
		WHERE setting_key = ?;
	`
)

// moveMealLeadDays moves stored lead counts between the keys of development
// and of the ordering rules, to calendar days (Up) or back to working days
// (Down). An explicit value at the destination wins, including zero; a
// rerun changes nothing.
func moveMealLeadDays(ctx context.Context, db bun.IDB, from, to string, toCalendarDays bool) error {
	var err error
	if toCalendarDays {
		_, err = db.NewRaw(workingToCalendarLeadDaysSQL, from, to, to, from).Exec(ctx)
	} else {
		_, err = db.NewRaw(calendarToWorkingLeadDaysSQL, from, to, to, from).Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("move meal lead days from %s: %w", from, err)
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
