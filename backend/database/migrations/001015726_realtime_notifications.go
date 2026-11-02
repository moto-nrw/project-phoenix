package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	realtimeNotificationsVersion     = "1.15.726"
	realtimeNotificationsDescription = "Benachrichtigungen für den Abruf durch andere API-Prozesse (#5504)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     realtimeNotificationsVersion,
		Description: realtimeNotificationsDescription,
		DependsOn: []string{
			staffQualificationTypesVersion, // extends its erasure functions
			holidayHealthDataVersion,       // follows the current migration head
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
		if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS platform.realtime_notifications`); err != nil {
			return fmt.Errorf("drop the realtime notifications: %w", err)
		}
		return nil
	})
}
