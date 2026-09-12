package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.384",
		Description: "Index append-only import batch checkpoints (#2708)",
		DependsOn:   []string{auditAppendOnlyGrantsVersion},
	})
	Migrations.MustRegister(importCheckpointsUp, importCheckpointsDown)
}

func importCheckpointsUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS uq_data_import_checkpoint
		ON audit.data_imports (tenant_id,
			(metadata -> 'import_checkpoint' ->> 'import_key'),
			(metadata -> 'import_checkpoint' ->> 'last_row'))
		WHERE metadata -> 'import_checkpoint' IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("create import checkpoint index: %w", err)
	}
	return nil
}

// Rollback keeps every checkpoint and audit row. Only the index is removed;
// reverting application wiring must never erase committed import evidence.
func importCheckpointsDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `DROP INDEX IF EXISTS audit.uq_data_import_checkpoint`)
	return err
}
