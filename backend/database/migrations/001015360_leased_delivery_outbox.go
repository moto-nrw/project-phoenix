package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	leasedDeliveryOutboxVersion     = "1.15.360"
	leasedDeliveryOutboxDescription = "Expand durable delivery into fenced email and push outboxes (#2657)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: leasedDeliveryOutboxVersion, Description: leasedDeliveryOutboxDescription,
		DependsOn: []string{auditCommandViewsVersion},
	})
	Migrations.MustRegister(leasedDeliveryOutboxUp, leasedDeliveryOutboxDown)
}

func leasedDeliveryOutboxUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.360: Expanding durable delivery into fenced email and push outboxes...")
	if _, err := db.NewRaw(`
		DROP INDEX IF EXISTS platform.idx_email_outbox_worker_pickup;

		ALTER TABLE platform.email_outbox
			DROP CONSTRAINT IF EXISTS email_outbox_status_check;
		ALTER TABLE platform.email_outbox
			ADD COLUMN recipient JSONB NOT NULL DEFAULT '{}'::jsonb,
			ADD COLUMN lease_token TEXT,
			ADD COLUMN lease_expires_at TIMESTAMPTZ,
			ADD COLUMN provider_result JSONB,
			ADD COLUMN dead_letter_at TIMESTAMPTZ,
			ADD COLUMN cancelled_at TIMESTAMPTZ;

		UPDATE platform.email_outbox
		SET recipient = jsonb_build_object('address', BTRIM(payload->>'recipient_email'))
		WHERE NULLIF(BTRIM(payload->>'recipient_email'), '') IS NOT NULL;
		ALTER TABLE platform.email_outbox
			ALTER COLUMN recipient DROP DEFAULT;

		UPDATE platform.email_outbox
		SET status = 'pending'
		WHERE status = 'sending';
		UPDATE platform.email_outbox
		SET status = 'dead_letter',
			last_error = 'legacy email outbox row has no recipient email',
			dead_letter_at = COALESCE(updated_at, NOW())
		WHERE status = 'pending'
			AND NULLIF(BTRIM(recipient->>'address'), '') IS NULL;
		UPDATE platform.email_outbox
		SET status = 'dead_letter',
			dead_letter_at = COALESCE(updated_at, NOW())
		WHERE status = 'failed';

		ALTER TABLE platform.email_outbox
			ADD CONSTRAINT email_outbox_status_check
			CHECK (status IN ('pending', 'claimed', 'sent', 'dead_letter', 'cancelled')),
			ADD CONSTRAINT email_outbox_lease_check
			CHECK (
				(status = 'claimed' AND lease_token IS NOT NULL AND lease_expires_at IS NOT NULL)
				OR (status <> 'claimed' AND lease_token IS NULL AND lease_expires_at IS NULL)
			);

		CREATE INDEX idx_email_outbox_worker_pickup
			ON platform.email_outbox (next_retry_at, id)
			WHERE status IN ('pending', 'claimed');

		CREATE TABLE platform.push_outbox (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			kind                TEXT NOT NULL,
			idempotency_key     TEXT,
			related_entity_type TEXT,
			related_entity_id   BIGINT,
			recipient           JSONB NOT NULL,
			payload             JSONB NOT NULL,
			status              TEXT NOT NULL DEFAULT 'pending'
				CHECK (status IN ('pending', 'claimed', 'sent', 'dead_letter', 'cancelled')),
			attempts            INT NOT NULL DEFAULT 0 CHECK (attempts >= 0),
			next_retry_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			lease_token         TEXT,
			lease_expires_at    TIMESTAMPTZ,
			provider_result     JSONB,
			last_error          TEXT,
			sent_at             TIMESTAMPTZ,
			dead_letter_at      TIMESTAMPTZ,
			cancelled_at        TIMESTAMPTZ,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT push_outbox_lease_check CHECK (
				(status = 'claimed' AND lease_token IS NOT NULL AND lease_expires_at IS NOT NULL)
				OR (status <> 'claimed' AND lease_token IS NULL AND lease_expires_at IS NULL)
			)
		);

		CREATE UNIQUE INDEX uq_push_outbox_tenant_idempotency
			ON platform.push_outbox (tenant_id, idempotency_key)
			WHERE idempotency_key IS NOT NULL;
		CREATE INDEX idx_push_outbox_worker_pickup
			ON platform.push_outbox (next_retry_at, id)
			WHERE status IN ('pending', 'claimed');
		CREATE INDEX idx_push_outbox_related_entity
			ON platform.push_outbox (tenant_id, related_entity_type, related_entity_id)
			WHERE related_entity_id IS NOT NULL;

		CREATE VIEW platform.delivery_email_deliveries
		WITH (security_invoker = true, security_barrier = true) AS
		SELECT * FROM platform.email_delivery;

		CREATE VIEW platform.delivery_push_subscriptions
		WITH (security_invoker = true, security_barrier = true) AS
		SELECT
			"push_subscription".*,
			COALESCE("account".active, FALSE) AS account_active,
			COALESCE("account_tenant".status = 'active', FALSE) AS tenant_active,
			EXISTS (
				SELECT 1
				FROM auth.account_roles AS "staff_account_role"
				INNER JOIN auth.roles AS "staff_role" ON "staff_role".id = "staff_account_role".role_id
				WHERE "staff_account_role".account_id = "push_subscription".account_id
					AND "staff_account_role".tenant_id = "push_subscription".tenant_id
					AND LOWER("staff_role".name) <> 'guardian'
			) AS has_staff_role,
			EXISTS (
				SELECT 1
				FROM auth.account_roles AS "school_account_role"
				INNER JOIN auth.roles AS "school_role" ON "school_role".id = "school_account_role".role_id
				WHERE "school_account_role".account_id = "push_subscription".account_id
					AND "school_account_role".tenant_id = "push_subscription".tenant_id
					AND "school_role".is_system
					AND LOWER(BTRIM("school_role".name)) = 'lehrkraft'
			) AS has_school_role,
			EXISTS (
				SELECT 1
				FROM auth.account_roles AS "guardian_account_role"
				INNER JOIN auth.roles AS "guardian_role" ON "guardian_role".id = "guardian_account_role".role_id
				WHERE "guardian_account_role".account_id = "push_subscription".account_id
					AND "guardian_account_role".tenant_id = "push_subscription".tenant_id
					AND LOWER("guardian_role".name) = 'guardian'
			) AS has_guardian_role,
			(
				EXISTS (
					SELECT 1
					FROM auth.account_roles AS "admin_account_role"
					INNER JOIN auth.roles AS "admin_role" ON "admin_role".id = "admin_account_role".role_id
					LEFT JOIN auth.role_permissions AS "role_permission" ON "role_permission".role_id = "admin_account_role".role_id
					LEFT JOIN auth.permissions AS "permission" ON "permission".id = "role_permission".permission_id
					WHERE "admin_account_role".account_id = "push_subscription".account_id
						AND "admin_account_role".tenant_id = "push_subscription".tenant_id
						AND (
							LOWER("admin_role".name) = 'admin'
							OR ("permission".resource = 'admin' AND "permission".action = '*')
							OR ("permission".resource = '*' AND "permission".action = '*')
						)
				) OR EXISTS (
					SELECT 1
					FROM auth.account_permissions AS "account_permission"
					INNER JOIN auth.permissions AS "permission" ON "permission".id = "account_permission".permission_id
					WHERE "account_permission".account_id = "push_subscription".account_id
						AND "account_permission".tenant_id = "push_subscription".tenant_id
						AND "account_permission".granted
						AND (
							("permission".resource = 'admin' AND "permission".action = '*')
							OR ("permission".resource = '*' AND "permission".action = '*')
						)
				)
			) AS effective_admin,
			COALESCE(ARRAY(
				SELECT "child_link".student_id
				FROM users.students_guardians AS "child_link"
				INNER JOIN users.guardian_profiles AS "child_guardian"
					ON "child_guardian".id = "child_link".guardian_profile_id
				WHERE "child_link".tenant_id = "push_subscription".tenant_id
					AND "child_guardian".account_id = "push_subscription".account_id
					AND "child_link".permissions @> '{"parent_portal.access": true}'::jsonb
			), '{}'::BIGINT[]) AS guardian_student_ids,
			CASE
				WHEN "push_subscription".token_family_id <> '' THEN EXISTS (
					SELECT 1 FROM auth.tokens AS "token"
					WHERE "token".account_id = "push_subscription".account_id
						AND "token".family_id = "push_subscription".token_family_id
						AND "token".rotated_at IS NULL AND "token".expiry > NOW()
				)
				ELSE EXISTS (
					SELECT 1 FROM auth.tokens AS "token"
					WHERE "token".account_id = "push_subscription".account_id
						AND "token".rotated_at IS NULL AND "token".expiry > NOW()
						AND (
							("push_subscription".portal = 'parent' AND "token".portal_scope IN ('parent', 'unknown', ''))
							OR ("push_subscription".portal = 'staff' AND "token".tenant_id = "push_subscription".tenant_id AND "token".portal_scope IN ('tenant', 'org', 'unknown', ''))
							OR ("push_subscription".portal = 'school' AND "token".tenant_id = "push_subscription".tenant_id AND "token".portal_scope IN ('school', 'unknown', ''))
						)
				)
			END AS has_live_token
		FROM iot.push_subscriptions AS "push_subscription"
		LEFT JOIN auth.accounts AS "account" ON "account".id = "push_subscription".account_id
		LEFT JOIN auth.account_tenants AS "account_tenant"
			ON "account_tenant".account_id = "push_subscription".account_id
			AND "account_tenant".tenant_id = "push_subscription".tenant_id;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("expand leased delivery outboxes: %w", err)
	}

	if err := provisionTenantRLS(ctx, db, "platform.push_outbox"); err != nil {
		return fmt.Errorf("provision push outbox RLS: %w", err)
	}
	if _, err := db.NewRaw(`
		GRANT SELECT, INSERT, UPDATE ON platform.push_outbox TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE platform.push_outbox_id_seq TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE, DELETE ON platform.delivery_email_deliveries TO phoenix_tenant;
		GRANT SELECT ON platform.delivery_push_subscriptions TO phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("grant push outbox permissions: %w", err)
	}
	return nil
}

func leasedDeliveryOutboxDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.360: Restoring the legacy email outbox schema...")
	if _, err := db.NewRaw(`
		DROP VIEW IF EXISTS platform.delivery_email_deliveries;
		DROP VIEW IF EXISTS platform.delivery_push_subscriptions;
		DROP TABLE IF EXISTS platform.push_outbox;
		DROP INDEX IF EXISTS platform.idx_email_outbox_worker_pickup;

		ALTER TABLE platform.email_outbox
			DROP CONSTRAINT IF EXISTS email_outbox_lease_check,
			DROP CONSTRAINT IF EXISTS email_outbox_status_check;
		UPDATE platform.email_outbox
		SET status = CASE
			WHEN status = 'claimed' THEN 'pending'
			WHEN status IN ('dead_letter', 'cancelled') THEN 'failed'
			ELSE status
		END;
		ALTER TABLE platform.email_outbox
			DROP COLUMN IF EXISTS recipient,
			DROP COLUMN IF EXISTS lease_token,
			DROP COLUMN IF EXISTS lease_expires_at,
			DROP COLUMN IF EXISTS provider_result,
			DROP COLUMN IF EXISTS dead_letter_at,
			DROP COLUMN IF EXISTS cancelled_at,
			ADD CONSTRAINT email_outbox_status_check
			CHECK (status IN ('pending', 'sending', 'sent', 'failed'));

		CREATE INDEX idx_email_outbox_worker_pickup
			ON platform.email_outbox (next_retry_at)
			WHERE status = 'pending';
	`).Exec(ctx); err != nil {
		return fmt.Errorf("restore legacy email outbox schema: %w", err)
	}
	return nil
}
