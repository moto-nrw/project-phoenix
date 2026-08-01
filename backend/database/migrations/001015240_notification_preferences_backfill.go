package migrations

import (
	"context"
	"fmt"

	authRepository "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	"github.com/uptrace/bun"
)

const (
	notificationPreferencesBackfillVersion     = "1.15.240"
	notificationPreferencesBackfillDescription = "Backfill notification consent for devices that already registered for push"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     notificationPreferencesBackfillVersion,
		Description: notificationPreferencesBackfillDescription,
		DependsOn:   []string{notificationPreferencesVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return notificationPreferencesBackfillUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return notificationPreferencesBackfillDown(ctx, db)
		},
	)
}

// dispatchWasEnabledSQL restricts the backfill to schools that had switched
// notifications.dispatch_enabled ON themselves, by carrying an explicit
// override row in config.setting_values.
//
// This is the whole justification for writing consent on somebody's behalf: at
// those schools the registered device really did receive these notifications,
// so the consent already existed in practice and is only being written down. A
// school that never switched dispatch on delivered nothing, so there is no
// prior consent to record. Adding a row there would be invented consent, and
// this release flips the registry default to true, so it would arrive together
// with a switch that suddenly reads "on" for a person who never agreed to any
// category.
//
// No override row and an explicit false are treated the same on purpose: both
// mean the school was running with dispatch off, which is what the registry
// default was until this release.
//
// The value comparison is 'true'::jsonb because config.setting_values.value is
// jsonb (1.15.25) and the settings service stores booleans via json.Marshal, so
// a boolean setting is the jsonb literal true, never the string "true". Same
// form as 1.15.98 and 1.15.125.
//
// The key is a string literal, not models/config.KeyNotificationsDispatchEnabled,
// for the same reason as the type strings below: migrations are frozen history.
const dispatchWasEnabledSQL = `EXISTS (
		SELECT 1
		FROM config.setting_values AS "dispatch"
		WHERE "dispatch".tenant_id = "sub".tenant_id
		  AND "dispatch".setting_key = 'notifications.dispatch_enabled'
		  AND "dispatch".value = 'true'::jsonb
	)`

// notificationPreferencesBackfillUp gives everyone whose registered Web Push
// device was actually being delivered to the consent that matches what that
// device used to deliver.
//
// Consent is stored as "a row means yes, no row means no" (1.15.239), and this
// release switches notifications.dispatch_enabled on by default while routing
// every producer through consent. Without this backfill, the two changes
// together would silence every phone that is receiving notifications today, in
// the same minute, with nothing on screen to explain it.
//
// Both blocks are therefore limited to schools that had dispatch switched on
// (see dispatchWasEnabledSQL) and to accounts that are still active members of
// that school. A device belonging to a deactivated account or to a family that
// has left the school never receives anything again, so a consent row for it
// would be a claim about a person's wishes that nobody ever made, kept forever:
// down() is a deliberate no-op, so nothing removes it later.
//
// Guardians get the single parent type. Only effective admins get the four
// reminder types, because the old reminders_due event used ScopeAdmin; another
// staff member's registered device never received it. The personal types added
// by this epic (my_activity_starting, student_absence_reported) are deliberately
// NOT backfilled: no existing registration says anything about them, and
// inventing consent is worse than an empty switch the person can turn on.
//
// ON CONFLICT DO NOTHING because an explicit opt-out is a decision: a row with
// enabled = false means somebody said no, and a backfill must never overwrite
// that. (None can exist yet, but the guard states the intent.)
//
// The type strings are literals here rather than constants: migrations are
// frozen history and must keep working after a rename. The catalogue lives in
// services/notifications/types.go; a renamed key simply leaves these rows inert,
// which 1.15.239 already documents.
func notificationPreferencesBackfillUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.240: Backfilling notification consent from existing push subscriptions...")

	guardianBackfillQuery := fmt.Sprintf(`
		INSERT INTO users.notification_preferences (tenant_id, account_id, notification_type, enabled)
		SELECT DISTINCT "sub".tenant_id, "sub".account_id, 'parent_announcement', TRUE
		FROM iot.push_subscriptions AS "sub"
		INNER JOIN auth.accounts AS "account"
			ON "account".id = "sub".account_id
		INNER JOIN auth.account_tenants AS "account_tenant"
			ON "account_tenant".account_id = "sub".account_id
			AND "account_tenant".tenant_id = "sub".tenant_id
		WHERE "sub".portal = 'parent'
		  AND "account".active = TRUE
		  AND "account_tenant".status = 'active'
		  AND %s
		ON CONFLICT (tenant_id, account_id, notification_type) DO NOTHING;
	`, dispatchWasEnabledSQL)
	if _, err := db.ExecContext(ctx, guardianBackfillQuery); err != nil {
		return fmt.Errorf("error backfilling guardian notification consent: %w", err)
	}

	staffBackfillQuery := fmt.Sprintf(`
		INSERT INTO users.notification_preferences (tenant_id, account_id, notification_type, enabled)
		SELECT DISTINCT "sub".tenant_id, "sub".account_id, "type", TRUE
		FROM iot.push_subscriptions AS "sub"
		INNER JOIN auth.accounts AS "account"
			ON "account".id = "sub".account_id
		INNER JOIN auth.account_tenants AS "account_tenant"
			ON "account_tenant".account_id = "sub".account_id
			AND "account_tenant".tenant_id = "sub".tenant_id
		CROSS JOIN unnest(ARRAY[
			'pickup_upcoming',
			'pickup_overdue',
			'activity_start',
			'activity_overdue'
		]) AS "type"
		WHERE "sub".portal = 'staff'
		  AND "account".active = TRUE
		  AND "account_tenant".status = 'active'
		  AND EXISTS (
			SELECT 1
			FROM auth.account_roles AS "staff_account_role"
			INNER JOIN auth.roles AS "staff_role"
				ON "staff_role".id = "staff_account_role".role_id
			WHERE "staff_account_role".account_id = "sub".account_id
			  AND "staff_account_role".tenant_id = "sub".tenant_id
			  AND LOWER("staff_role".name) <> 'guardian'
		  )
		  AND (%s)
		  AND %s
		ON CONFLICT (tenant_id, account_id, notification_type) DO NOTHING;
	`, authRepository.EffectiveAdminExistsSQL(`"sub".account_id`, `"sub".tenant_id`), dispatchWasEnabledSQL)
	if _, err := db.ExecContext(ctx, staffBackfillQuery); err != nil {
		return fmt.Errorf("error backfilling staff notification consent: %w", err)
	}

	return nil
}

// notificationPreferencesBackfillDown is a deliberate no-op, like 1.15.234's.
//
// Once the rows exist they are indistinguishable from a choice the person made
// themselves in their profile, and deleting somebody's own switch to undo a
// migration is the worse failure. Dropping the table entirely is what 1.15.239
// is for.
func notificationPreferencesBackfillDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back 1.15.240: nothing to do (backfilled consent is indistinguishable from a user's own choice)")
	return nil
}
