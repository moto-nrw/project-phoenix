package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addEmailOutboxIdempotencyVersion     = "1.15.178"
	addEmailOutboxIdempotencyDescription = "Add tenant-scoped email outbox idempotency keys"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addEmailOutboxIdempotencyVersion,
		Description: addEmailOutboxIdempotencyDescription,
		DependsOn:   []string{"1.15.176"},
	})
	Migrations.MustRegister(addEmailOutboxIdempotencyUp, addEmailOutboxIdempotencyDown)
}

func addEmailOutboxIdempotencyUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.178: Adding email outbox idempotency keys...")
	_, err := db.NewRaw(`
		ALTER TABLE platform.email_outbox ADD COLUMN idempotency_key TEXT;
		CREATE UNIQUE INDEX uq_email_outbox_tenant_idempotency
			ON platform.email_outbox (tenant_id, idempotency_key)
			WHERE idempotency_key IS NOT NULL;
	`).Exec(ctx)
	return err
}

func addEmailOutboxIdempotencyDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.178: Removing email outbox idempotency keys...")
	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS platform.uq_email_outbox_tenant_idempotency;
		ALTER TABLE platform.email_outbox DROP COLUMN IF EXISTS idempotency_key;
	`).Exec(ctx)
	return err
}
