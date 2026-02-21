package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	securityInvokerViewsVersion     = "1.15.5"
	securityInvokerViewsDescription = "Add security_invoker=true to views on tenant-scoped tables (RLS bypass prevention)"
)

func init() {
	MigrationRegistry[securityInvokerViewsVersion] = &Migration{
		Version:     securityInvokerViewsVersion,
		Description: securityInvokerViewsDescription,
		DependsOn:   []string{"1.15.4"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.5: Adding security_invoker=true to views on tenant-scoped tables...")

			// Recreate expired_privacy_consents view with security_invoker = true.
			// Without this, the view executes as its owner and bypasses RLS policies,
			// exposing data from ALL tenants regardless of the current session's tenant_id.
			//
			// We must DROP first because pc.* now includes tenant_id (added by
			// Phase 4 migrations), which changes the column list. CREATE OR REPLACE
			// cannot alter existing column names/order.
			_, err := db.ExecContext(ctx, `
				DROP VIEW IF EXISTS users.expired_privacy_consents;
				CREATE VIEW users.expired_privacy_consents
				WITH (security_invoker = true) AS
				SELECT
					pc.*,
					s.person_id,
					s.guardian_name,
					s.guardian_email,
					s.guardian_phone
				FROM users.privacy_consents pc
				JOIN users.students s ON pc.student_id = s.id
				WHERE pc.expires_at < CURRENT_TIMESTAMP
				  AND pc.accepted = TRUE
				  AND pc.renewal_required = TRUE;
			`)
			if err != nil {
				return fmt.Errorf("error adding security_invoker to expired_privacy_consents view: %w", err)
			}

			fmt.Println("Migration 1.15.5: security_invoker=true applied to all views")
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.5: Removing security_invoker from views...")

			// Recreate view without security_invoker (back to default SECURITY DEFINER)
			_, err := db.ExecContext(ctx, `
				DROP VIEW IF EXISTS users.expired_privacy_consents;
				CREATE VIEW users.expired_privacy_consents AS
				SELECT
					pc.*,
					s.person_id,
					s.guardian_name,
					s.guardian_email,
					s.guardian_phone
				FROM users.privacy_consents pc
				JOIN users.students s ON pc.student_id = s.id
				WHERE pc.expires_at < CURRENT_TIMESTAMP
				  AND pc.accepted = TRUE
				  AND pc.renewal_required = TRUE;
			`)
			if err != nil {
				return fmt.Errorf("error removing security_invoker from expired_privacy_consents view: %w", err)
			}

			fmt.Println("Migration 1.15.5: Rolled back security_invoker changes")
			return nil
		},
	)
}
