package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	emailDeliveryEventsVersion     = "1.15.227"
	emailDeliveryEventsDescription = "Add provider delivery-status tracking to platform.email_outbox and create platform.email_delivery_events (#1937)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     emailDeliveryEventsVersion,
		Description: emailDeliveryEventsDescription,
		DependsOn: []string{
			createEmailOutboxVersion,         // 1.15.58 — the table we extend
			addEmailOutboxIdempotencyVersion, // 1.15.178 — idempotency_key column
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return emailDeliveryEventsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return emailDeliveryEventsDown(ctx, db)
		},
	)
}

func emailDeliveryEventsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.227: Adding delivery-status tracking to platform.email_outbox...")

	// Delivery status is a SECOND, orthogonal axis to the existing `status`
	// column. `status` answers "did we hand the message to the SMTP relay";
	// `delivery_status` answers "what did the receiving side do with it".
	// Conflating them is what makes today's UI claim "gesendet" for a mail
	// that never reached the recipient.
	//
	// delivery_status_rank is denormalised from delivery_status so the
	// out-of-order guard in ApplyDeliveryStatus is a single race-free UPDATE
	// instead of a read-modify-write.
	if _, err := db.NewRaw(`
		ALTER TABLE platform.email_outbox
			ADD COLUMN IF NOT EXISTS message_id           TEXT,
			ADD COLUMN IF NOT EXISTS provider_message_id  TEXT,
			ADD COLUMN IF NOT EXISTS delivery_status      TEXT NOT NULL DEFAULT 'unknown',
			ADD COLUMN IF NOT EXISTS delivery_status_rank SMALLINT NOT NULL DEFAULT 0,
			ADD COLUMN IF NOT EXISTS delivery_status_at   TIMESTAMPTZ,
			ADD COLUMN IF NOT EXISTS delivery_detail      TEXT;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding delivery columns to email_outbox: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE platform.email_outbox
			DROP CONSTRAINT IF EXISTS email_outbox_delivery_status_check;
		ALTER TABLE platform.email_outbox
			ADD CONSTRAINT email_outbox_delivery_status_check
			CHECK (delivery_status IN ('unknown', 'queued', 'deferred', 'delivered',
			                           'soft_bounced', 'suppressed', 'complained', 'bounced'));
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding delivery_status check constraint: %w", err)
	}

	// Correlation indexes. Not tenant-prefixed on purpose: the webhook is
	// unauthenticated and cross-tenant, so it looks a message up before any
	// tenant is known. Uniqueness is what makes that lookup unambiguous.
	if _, err := db.NewRaw(`
		CREATE UNIQUE INDEX IF NOT EXISTS uq_email_outbox_message_id
			ON platform.email_outbox (message_id) WHERE message_id IS NOT NULL;
		CREATE UNIQUE INDEX IF NOT EXISTS uq_email_outbox_provider_message_id
			ON platform.email_outbox (provider_message_id) WHERE provider_message_id IS NOT NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating message-id indexes: %w", err)
	}

	fmt.Println("Migration 1.15.227: Creating platform.email_dispatch_attempts...")

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS platform.email_dispatch_attempts (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			outbox_id           BIGINT NOT NULL REFERENCES platform.email_outbox(id) ON DELETE CASCADE,
			attempt_number      INTEGER NOT NULL CHECK (attempt_number > 0),
			correlation_token   UUID NOT NULL,
			message_id          TEXT NOT NULL,
			provider_message_id TEXT,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (outbox_id, attempt_number)
		);

		CREATE UNIQUE INDEX IF NOT EXISTS uq_email_dispatch_attempts_message_id
			ON platform.email_dispatch_attempts (message_id);
		CREATE UNIQUE INDEX IF NOT EXISTS uq_email_dispatch_attempts_provider_message_id
			ON platform.email_dispatch_attempts (provider_message_id)
			WHERE provider_message_id IS NOT NULL;
		CREATE UNIQUE INDEX IF NOT EXISTS uq_email_dispatch_attempts_correlation_token
			ON platform.email_dispatch_attempts (correlation_token);
		CREATE INDEX IF NOT EXISTS idx_email_dispatch_attempts_outbox
			ON platform.email_dispatch_attempts (outbox_id, attempt_number DESC);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating platform.email_dispatch_attempts: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE platform.email_dispatch_attempts ENABLE ROW LEVEL SECURITY;
		ALTER TABLE platform.email_dispatch_attempts FORCE ROW LEVEL SECURITY;
		DROP POLICY IF EXISTS tenant_isolation_platform_email_dispatch_attempts
			ON platform.email_dispatch_attempts;
		CREATE POLICY tenant_isolation_platform_email_dispatch_attempts
			ON platform.email_dispatch_attempts
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed enabling RLS on email_dispatch_attempts: %w", err)
	}

	fmt.Println("Migration 1.15.227: Creating platform.email_delivery_events...")

	// outbox_id is NOT NULL by design: it makes tenant_id derivable and
	// un-forgeable. An event we cannot match to an outbox row is never
	// persisted — it is counted and logged instead (there would be no tenant
	// for RLS to scope it to).
	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS platform.email_delivery_events (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			outbox_id           BIGINT NOT NULL REFERENCES platform.email_outbox(id) ON DELETE CASCADE,
			dispatch_attempt_id BIGINT NOT NULL REFERENCES platform.email_dispatch_attempts(id) ON DELETE CASCADE,
			provider            TEXT NOT NULL,
			provider_event_id   TEXT NOT NULL,
			provider_message_id TEXT,
			event_type          TEXT NOT NULL
				CHECK (event_type IN ('queued', 'delivered', 'deferred', 'bounced',
				                      'complained', 'suppressed')),
			bounce_class        TEXT CHECK (bounce_class IN ('hard', 'soft')),
			occurred_at         TIMESTAMPTZ NOT NULL,
			received_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			reason_code         TEXT,
			reason              TEXT,
			recipient           TEXT,
			raw                 JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating platform.email_delivery_events: %w", err)
	}

	if _, err := db.NewRaw(`
		-- Wire-level dedupe key. Global rather than tenant-scoped: the webhook
		-- is unauthenticated, so a replayed event must not be re-applicable
		-- under a second tenant.
		CREATE UNIQUE INDEX IF NOT EXISTS uq_email_delivery_events_provider_event
			ON platform.email_delivery_events (provider, provider_event_id);

		-- Tenant read API: "events for these outbox rows, newest first".
		CREATE INDEX IF NOT EXISTS idx_email_delivery_events_outbox
			ON platform.email_delivery_events (outbox_id, occurred_at DESC);
		CREATE INDEX IF NOT EXISTS idx_email_delivery_events_dispatch_attempt
			ON platform.email_delivery_events (dispatch_attempt_id, occurred_at DESC);

		-- Ops triage / dashboards.
		CREATE INDEX IF NOT EXISTS idx_email_delivery_events_tenant_type
			ON platform.email_delivery_events (tenant_id, event_type, occurred_at DESC);

		-- Retention sweep.
		CREATE INDEX IF NOT EXISTS idx_email_delivery_events_received_at
			ON platform.email_delivery_events (received_at);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating email_delivery_events indexes: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE platform.email_delivery_events ENABLE ROW LEVEL SECURITY;
		ALTER TABLE platform.email_delivery_events FORCE ROW LEVEL SECURITY;
		DROP POLICY IF EXISTS tenant_isolation_platform_email_delivery_events
			ON platform.email_delivery_events;
		CREATE POLICY tenant_isolation_platform_email_delivery_events
			ON platform.email_delivery_events
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed enabling RLS on email_delivery_events: %w", err)
	}

	// Read-only for tenant requests. Every write path (webhook ingest,
	// retention sweep) runs as phoenix_admin, which already holds ALL on
	// platform.* via the 1.14.1 default privileges. Deliberately narrower
	// than the email_outbox grants — no INSERT, so no sequence USAGE either.
	if _, err := db.NewRaw(`
		GRANT SELECT ON platform.email_delivery_events TO phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed granting permissions on email_delivery_events: %w", err)
	}

	return nil
}

func emailDeliveryEventsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.227: Dropping email delivery tracking...")

	if _, err := db.NewRaw(`DROP TABLE IF EXISTS platform.email_delivery_events CASCADE;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping platform.email_delivery_events: %w", err)
	}
	if _, err := db.NewRaw(`DROP TABLE IF EXISTS platform.email_dispatch_attempts CASCADE;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping platform.email_dispatch_attempts: %w", err)
	}

	if _, err := db.NewRaw(`
		DROP INDEX IF EXISTS platform.uq_email_outbox_message_id;
		DROP INDEX IF EXISTS platform.uq_email_outbox_provider_message_id;
		ALTER TABLE platform.email_outbox
			DROP CONSTRAINT IF EXISTS email_outbox_delivery_status_check;
		ALTER TABLE platform.email_outbox
			DROP COLUMN IF EXISTS message_id,
			DROP COLUMN IF EXISTS provider_message_id,
			DROP COLUMN IF EXISTS delivery_status,
			DROP COLUMN IF EXISTS delivery_status_rank,
			DROP COLUMN IF EXISTS delivery_status_at,
			DROP COLUMN IF EXISTS delivery_detail;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping delivery columns from email_outbox: %w", err)
	}

	return nil
}
