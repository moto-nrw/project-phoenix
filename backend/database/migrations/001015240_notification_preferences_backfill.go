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

// notificationPreferencesBackfillUp gives everyone who already registered a
// device for Web Push the consent that matches what that device used to
// deliver.
//
// Consent is stored as "a row means yes, no row means no" (1.15.239), and this
// release switches notifications.dispatch_enabled on by default while routing
// every producer through consent. Without this backfill, the two changes
// together would silence every phone that is registered today, in the same
// minute, with nothing on screen to explain it.
//
// Guardians get the single parent type. Only active effective admins get the
// four reminder types, because the old reminders_due event used ScopeAdmin;
// another staff member's registered device never received it. The personal
// types added by this epic (my_activity_starting, student_absence_reported) are
// deliberately NOT backfilled: no existing registration says anything about
// them, and inventing consent is worse than an empty switch the person can turn
// on.
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

	if _, err := db.ExecContext(ctx, `
		INSERT INTO users.notification_preferences (tenant_id, account_id, notification_type, enabled)
		SELECT DISTINCT "sub".tenant_id, "sub".account_id, 'parent_announcement', TRUE
		FROM iot.push_subscriptions AS "sub"
		WHERE "sub".portal = 'parent'
		ON CONFLICT (tenant_id, account_id, notification_type) DO NOTHING;
	`); err != nil {
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
		ON CONFLICT (tenant_id, account_id, notification_type) DO NOTHING;
	`, authRepository.EffectiveAdminExistsSQL(`"sub".account_id`, `"sub".tenant_id`))
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
