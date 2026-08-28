package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	activityInstanceIdempotencyVersion     = "1.15.337"
	activityInstanceIdempotencyDescription = "Add tenant-scoped idempotency keys for manual timetable instance creation (#2532)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     activityInstanceIdempotencyVersion,
		Description: activityInstanceIdempotencyDescription,
		DependsOn:   []string{createActivityInstancesVersion},
	})

	Migrations.MustRegister(activityInstanceIdempotencyUp, activityInstanceIdempotencyDown)
}

func activityInstanceIdempotencyUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.337: Adding activity-instance idempotency keys...")

	_, err := db.NewRaw(`
		ALTER TABLE schedule.activity_instances
			ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

		CREATE UNIQUE INDEX IF NOT EXISTS uq_activity_instances_tenant_idempotency
			ON schedule.activity_instances (tenant_id, idempotency_key)
			WHERE idempotency_key IS NOT NULL;

		COMMENT ON COLUMN schedule.activity_instances.idempotency_key IS
			'Client-generated key for replaying one create operation without inserting a second instance (#2532)';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("add activity-instance idempotency keys: %w", err)
	}
	return nil
}

func activityInstanceIdempotencyDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.337: Removing activity-instance idempotency keys...")

	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS schedule.uq_activity_instances_tenant_idempotency;
		ALTER TABLE schedule.activity_instances
			DROP COLUMN IF EXISTS idempotency_key;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("remove activity-instance idempotency keys: %w", err)
	}
	return nil
}
