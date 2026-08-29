package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	requestLegalBlocksSnapshotVersion     = "1.15.341"
	requestLegalBlocksSnapshotDescription = "Add append-only legal-blocks consent evidence snapshot to enrollment requests"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     requestLegalBlocksSnapshotVersion,
		Description: requestLegalBlocksSnapshotDescription,
		DependsOn: []string{
			"1.15.60", // enrollment.requests
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.341: Adding legal_blocks_snapshot to enrollment.requests...")
			// No backfill: the wording shown to families before this
			// migration cannot be reconstructed reliably, so historical
			// requests keep an empty snapshot instead of a fabricated one.
			_, err := db.NewRaw(`
				ALTER TABLE enrollment.requests
					ADD COLUMN IF NOT EXISTS legal_blocks_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb;
			`).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed adding requests.legal_blocks_snapshot: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.341: Dropping requests.legal_blocks_snapshot...")
			_, err := db.NewRaw(`
				ALTER TABLE enrollment.requests
					DROP COLUMN IF EXISTS legal_blocks_snapshot;
			`).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed dropping requests.legal_blocks_snapshot: %w", err)
			}
			return nil
		},
	)
}
