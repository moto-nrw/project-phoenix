package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	realtimeNotificationsVersion     = "1.15.727"
	realtimeNotificationsDescription = "Benachrichtigungen für den Abruf durch andere API-Prozesse (#5504)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     realtimeNotificationsVersion,
		Description: realtimeNotificationsDescription,
		DependsOn: []string{
			staffQualificationTypesVersion,     // extends its erasure functions
			holidayHealthDataVersion,           // follows the holiday health data
			authTokenSwitchedFromFamilyVersion, // follows the current migration head
		},
	})
	Migrations.MustRegister(realtimeNotificationsUp, realtimeNotificationsDown)
}

// realtimeNotificationsUpSQL adds the short-lived copy of an in-app
// notification (#5504). The PostgreSQL relay between processes is not
// authenticated, so it carries only the row's ID with school and address.
// The receiving API reads the row in the school's transaction and shows it
// only to the recipients stored here:
//
//   - scope is the audience of the notification; account_ids holds the
//     addressed staff accounts or the guardian accounts that passed the
//     child-access check; active_group_id the addressed active group.
//   - staff_portal / school_portal say which staff portal the accounts were
//     addressed in.
//   - content is the display-safe SSE payload (title, body, app-relative
//     links, priority, type and opaque routing IDs).
//
// A row is readable for two minutes; the nightly Delivery retention deletes
// it afterwards.
//
// platform.delivery_realtime_recipients is Delivery's projection for showing
// a relayed copy: it answers, per account and school, the delivery-time rules
// of platform.delivery_push_subscriptions (active account and membership,
// staff, school-portal or guardian role, children with parent_portal.access)
// for accounts that have no push subscription. Its role and child columns
// copy the expressions of the push view (1.15.480) verbatim, including school
// roles marked school_portal; a migration test compares them.
const realtimeNotificationsUpSQL = `
		CREATE TABLE platform.realtime_notifications (
			id              BIGSERIAL PRIMARY KEY,
			tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			scope           TEXT NOT NULL CHECK (scope IN ('tenant', 'admin', 'group', 'staff', 'guardian')),
			active_group_id TEXT NOT NULL DEFAULT '',
			account_ids     BIGINT[] NOT NULL DEFAULT '{}',
			staff_portal    BOOLEAN NOT NULL DEFAULT FALSE,
			school_portal   BOOLEAN NOT NULL DEFAULT FALSE,
			content         JSONB NOT NULL,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX idx_realtime_notifications_tenant_created
			ON platform.realtime_notifications (tenant_id, created_at);
		COMMENT ON TABLE platform.realtime_notifications IS
			'In-App-Benachrichtigungen für andere API-Prozesse (#5504): Inhalt und Empfänger, zwei Minuten lesbar, danach von der Delivery-Aufbewahrung gelöscht.';

		REVOKE ALL ON platform.realtime_notifications FROM PUBLIC, phoenix_tenant;
		GRANT SELECT, INSERT, DELETE ON platform.realtime_notifications TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE platform.realtime_notifications_id_seq TO phoenix_tenant;

		CREATE VIEW platform.delivery_realtime_recipients
		WITH (security_invoker = true, security_barrier = true) AS
		SELECT
			"account_tenant".tenant_id,
			"account_tenant".account_id,
			COALESCE("account".active, FALSE) AND "account_tenant".status = 'active' AS active,
			EXISTS (
				SELECT 1
				FROM auth.account_roles AS "staff_account_role"
				INNER JOIN auth.roles AS "staff_role" ON "staff_role".id = "staff_account_role".role_id
				WHERE "staff_account_role".account_id = "account_tenant".account_id
					AND "staff_account_role".tenant_id = "account_tenant".tenant_id
					AND LOWER("staff_role".name) <> 'guardian'
			) AS has_staff_role,
			EXISTS (
				SELECT 1
				FROM auth.account_roles AS "school_account_role"
				INNER JOIN auth.roles AS "school_role" ON "school_role".id = "school_account_role".role_id
				WHERE "school_account_role".account_id = "account_tenant".account_id
					AND "school_account_role".tenant_id = "account_tenant".tenant_id
					AND (("school_role".is_system
						AND LOWER(BTRIM("school_role".name)) = 'lehrkraft')
						OR "school_role".school_portal)
			) AS has_school_role,
			EXISTS (
				SELECT 1
				FROM auth.account_roles AS "guardian_account_role"
				INNER JOIN auth.roles AS "guardian_role" ON "guardian_role".id = "guardian_account_role".role_id
				WHERE "guardian_account_role".account_id = "account_tenant".account_id
					AND "guardian_account_role".tenant_id = "account_tenant".tenant_id
					AND LOWER("guardian_role".name) = 'guardian'
			) AS has_guardian_role,
			COALESCE(ARRAY(
				SELECT "child_link".student_id
				FROM users.student_guardian_relationships AS "child_link"
				INNER JOIN auth.guardian_student_access AS "child_access"
					ON "child_access".tenant_id = "child_link".tenant_id
					AND "child_access".relationship_id = "child_link".id
				INNER JOIN users.guardian_profiles AS "child_guardian"
					ON "child_guardian".id = "child_link".guardian_profile_id
				WHERE "child_link".tenant_id = "account_tenant".tenant_id
					AND "child_guardian".account_id = "account_tenant".account_id
					AND "child_access".permissions @> '{"parent_portal.access": true}'::jsonb
			), '{}'::BIGINT[]) AS guardian_student_ids
		FROM auth.account_tenants AS "account_tenant"
		LEFT JOIN auth.accounts AS "account" ON "account".id = "account_tenant".account_id;
		GRANT SELECT ON platform.delivery_realtime_recipients TO phoenix_tenant;
`

func realtimeNotificationsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, realtimeNotificationsUpSQL); err != nil {
			return fmt.Errorf("create the realtime notifications: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, "platform.realtime_notifications"); err != nil {
			return err
		}
		for _, step := range []func(context.Context, bun.Tx) error{
			installRealtimeNotificationErasurePlan, installRealtimeNotificationErasurePreflight, installRealtimeNotificationErasureResidual,
			installRealtimeNotificationErasureDelete, installRealtimeNotificationErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("install the realtime notification school erasure: %w", err)
			}
		}
		return nil
	})
}

// realtimeNotificationsDown restores the 1.15.724 erasure functions before it
// drops the table, so the installed plan never names a missing table.
func realtimeNotificationsDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range []func(context.Context, bun.Tx) error{
			installStaffQualificationTypeErasurePlan, installStaffQualificationTypeErasurePreflight, installStaffQualificationTypeErasureResidual,
			installStaffQualificationTypeErasureDelete, installStaffQualificationTypeErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("restore the staff qualification type school erasure: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, `DROP VIEW IF EXISTS platform.delivery_realtime_recipients;
			DROP TABLE IF EXISTS platform.realtime_notifications`); err != nil {
			return fmt.Errorf("drop the realtime notifications: %w", err)
		}
		return nil
	})
}
