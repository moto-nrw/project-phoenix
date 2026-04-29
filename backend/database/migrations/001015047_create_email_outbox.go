package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createEmailOutboxVersion     = "1.15.47"
	createEmailOutboxDescription = "Create platform.email_outbox table for shared async email delivery (parent-enrollment PR 5)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createEmailOutboxVersion,
		Description: createEmailOutboxDescription,
		DependsOn: []string{
			"1.15.1", // RLS infrastructure
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createEmailOutboxUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return createEmailOutboxDown(ctx, db)
		},
	)
}

func createEmailOutboxUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.47: Creating platform.email_outbox table...")

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS platform.email_outbox (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			kind                TEXT NOT NULL,
			related_entity_type TEXT,
			related_entity_id   BIGINT,
			payload             JSONB NOT NULL DEFAULT '{}'::jsonb,
			status              TEXT NOT NULL DEFAULT 'pending'
				CHECK (status IN ('pending', 'sending', 'sent', 'failed')),
			attempts            INT NOT NULL DEFAULT 0,
			last_error          TEXT,
			next_retry_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			sent_at             TIMESTAMPTZ
		);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating platform.email_outbox: %w", err)
	}

	// Worker pickup index — partial covering only rows the worker can act on.
	// Pending and not-yet-due rows are quickly skipped.
	if _, err := db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_email_outbox_worker_pickup
		ON platform.email_outbox (next_retry_at)
		WHERE status = 'pending';
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating worker pickup index: %w", err)
	}

	// Admin/UI lookup by related entity (e.g., "show me all email rows for
	// enrollment_request 42").
	if _, err := db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_email_outbox_related_entity
		ON platform.email_outbox (tenant_id, related_entity_type, related_entity_id)
		WHERE related_entity_id IS NOT NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating related-entity index: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE platform.email_outbox ENABLE ROW LEVEL SECURITY;
		ALTER TABLE platform.email_outbox FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_platform_email_outbox ON platform.email_outbox
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed enabling RLS on email_outbox: %w", err)
	}

	// phoenix_tenant role needs INSERT/SELECT/UPDATE so request handlers
	// running in tenant tx can enqueue rows and read delivery status.
	// The worker uses phoenix_admin (cross-tenant pickup), which already has
	// ALL on platform.* via migration 1.14.1 default privileges.
	if _, err := db.NewRaw(`
		GRANT INSERT, SELECT, UPDATE ON platform.email_outbox TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE platform.email_outbox_id_seq TO phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed granting permissions on email_outbox: %w", err)
	}

	return nil
}

func createEmailOutboxDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.47: Dropping platform.email_outbox...")
	if _, err := db.NewRaw(`DROP TABLE IF EXISTS platform.email_outbox CASCADE;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping platform.email_outbox: %w", err)
	}
	return nil
}
