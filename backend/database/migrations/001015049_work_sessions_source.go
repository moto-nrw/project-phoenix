package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	workSessionsSourceVersion     = "1.15.49"
	workSessionsSourceDescription = "Add source column to active.work_sessions to distinguish App vs NFC vs auto-created stamps (Issue #1368)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     workSessionsSourceVersion,
		Description: workSessionsSourceDescription,
		DependsOn:   []string{workSessionsVersion}, // 1.10.1 — work_sessions table
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return workSessionsSourceUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return workSessionsSourceDown(ctx, db)
		},
	)
}

// workSessionsSourceUp adds a `source` column that records which channel
// produced a work session: 'app' (POST /api/time-tracking/check-in, used
// by the App and Web flow) or 'nfc' (auto-stamp triggered by a kiosk scan
// flowing through services/active session_service.assignSupervisorNonCritical).
// The default is 'app' because the App path is the only direct check-in
// path users invoke today; existing rows are backfilled with 'app' for the
// same reason.
func workSessionsSourceUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.49: Adding source column to active.work_sessions...")

	if _, err := db.NewRaw(`
		ALTER TABLE active.work_sessions
		ADD COLUMN IF NOT EXISTS source VARCHAR(10) NOT NULL DEFAULT 'app'
		CONSTRAINT chk_work_sessions_source CHECK (source IN ('app','nfc'));
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding source column: %w", err)
	}

	return nil
}

func workSessionsSourceDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.49: Dropping source column from active.work_sessions...")

	if _, err := db.NewRaw(`
		ALTER TABLE active.work_sessions DROP COLUMN IF EXISTS source;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping source column: %w", err)
	}

	return nil
}
