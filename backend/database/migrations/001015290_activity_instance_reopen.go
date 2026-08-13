package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const activityInstanceReopenVersion = "1.15.290"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     activityInstanceReopenVersion,
		Description: "Track the actor and five-minute undo window for completed activities (#2266)",
		DependsOn:   []string{parentCareRequestFieldSettingsVersion},
	})
	Migrations.MustRegister(activityInstanceReopenUp, activityInstanceReopenDown)
}

func activityInstanceReopenUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.290: Adding activity completion recovery metadata (#2266)...")
	_, err := db.NewRaw(`
		ALTER TABLE schedule.activity_instances
			ADD COLUMN IF NOT EXISTS completed_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			ADD COLUMN IF NOT EXISTS reopen_until TIMESTAMPTZ;
		CREATE INDEX IF NOT EXISTS idx_activity_instances_reopenable
			ON schedule.activity_instances (tenant_id, reopen_until)
			WHERE status = 'completed' AND reopen_until IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("add activity recovery metadata: %w", err)
	}
	return nil
}

func activityInstanceReopenDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS schedule.idx_activity_instances_reopenable;
		ALTER TABLE schedule.activity_instances DROP COLUMN IF EXISTS reopen_until, DROP COLUMN IF EXISTS completed_by;
	`).Exec(ctx)
	return err
}
