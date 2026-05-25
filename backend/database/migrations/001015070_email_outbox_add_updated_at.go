package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	emailOutboxAddUpdatedAtVersion     = "1.15.70"
	emailOutboxAddUpdatedAtDescription = "Add updated_at column to platform.email_outbox to satisfy base.Model embedding (parent-enrollment PR 5 followup)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     emailOutboxAddUpdatedAtVersion,
		Description: emailOutboxAddUpdatedAtDescription,
		DependsOn: []string{
			createEmailOutboxVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.NewRaw(`
				ALTER TABLE platform.email_outbox
				ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
			`).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed adding updated_at to email_outbox: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.NewRaw(`ALTER TABLE platform.email_outbox DROP COLUMN IF EXISTS updated_at;`).Exec(ctx)
			return err
		},
	)
}
