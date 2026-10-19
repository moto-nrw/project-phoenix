package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const systemEmailOutboxVersion = "1.15.461"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     systemEmailOutboxVersion,
		Description: "Support global identity mail in the leased email outbox",
		DependsOn:   []string{leasedDeliveryOutboxVersion, "1.15.178"},
	})
	Migrations.MustRegister(systemEmailOutboxUp, systemEmailOutboxDown)
}

func systemEmailOutboxUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE platform.email_outbox ALTER COLUMN tenant_id DROP NOT NULL;
		DROP INDEX platform.uq_email_outbox_tenant_idempotency;
		CREATE UNIQUE INDEX uq_email_outbox_tenant_idempotency
			ON platform.email_outbox (tenant_id, idempotency_key) NULLS NOT DISTINCT
			WHERE idempotency_key IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("expand system email outbox: %w", err)
	}
	// Existing tenant RLS excludes NULL, so only the privileged admin
	// transaction can insert or read global system mail.
	return nil
}

func systemEmailOutboxDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE platform.email_outbox ALTER COLUMN tenant_id SET NOT NULL;
		DROP INDEX platform.uq_email_outbox_tenant_idempotency;
		CREATE UNIQUE INDEX uq_email_outbox_tenant_idempotency
			ON platform.email_outbox (tenant_id, idempotency_key)
			WHERE idempotency_key IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("contract system email outbox (requires no global intents): %w", err)
	}
	return nil
}
