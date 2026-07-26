package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	workSessionsReopenedAtVersion     = "1.15.160"
	workSessionsReopenedAtDescription = "Track same-day work session reopen time for auto-checkout"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     workSessionsReopenedAtVersion,
		Description: workSessionsReopenedAtDescription,
		DependsOn:   []string{staffShiftsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return workSessionsReopenedAtUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return workSessionsReopenedAtDown(ctx, db)
		},
	)
}

func workSessionsReopenedAtUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.160: Adding reopened_at to active.work_sessions...")

	if _, err := db.NewRaw(`
		ALTER TABLE active.work_sessions
		ADD COLUMN IF NOT EXISTS reopened_at TIMESTAMPTZ;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding active.work_sessions.reopened_at: %w", err)
	}

	return nil
}

func workSessionsReopenedAtDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.160: Dropping active.work_sessions.reopened_at...")

	if _, err := db.NewRaw(`
		ALTER TABLE active.work_sessions
		DROP COLUMN IF EXISTS reopened_at;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping active.work_sessions.reopened_at: %w", err)
	}

	return nil
}
