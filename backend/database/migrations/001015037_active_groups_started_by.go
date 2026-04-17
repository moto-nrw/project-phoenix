package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	activeGroupsStartedByVersion     = "1.15.37"
	activeGroupsStartedByDescription = "Add started_by and started_by_source to active.groups to distinguish device-started vs web-started sessions"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     activeGroupsStartedByVersion,
		Description: activeGroupsStartedByDescription,
		DependsOn: []string{
			ActiveGroupsVersion, // 1.4.1 — active.groups
			UsersStaffVersion,   // 1.2.3 — users.staff
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addActiveGroupsStartedBy(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropActiveGroupsStartedBy(ctx, db)
		},
	)
}

// addActiveGroupsStartedBy adds:
//   - started_by (BIGINT, FK users.staff, NULL = unknown/backfilled)
//   - started_by_source (TEXT, CHECK 'device' | 'web', default 'device')
//
// Backfill: all existing rows default to 'device' because today every session
// originates from a PyrePortal kiosk. Rows with non-NULL device_id keep that
// provenance; new web-started sessions will set source='web' and device_id=NULL.
func addActiveGroupsStartedBy(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.37: Adding started_by + started_by_source to active.groups...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in active_groups_started_by migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.groups
			ADD COLUMN IF NOT EXISTS started_by BIGINT
				REFERENCES users.staff(id) ON DELETE SET NULL;

		ALTER TABLE active.groups
			ADD COLUMN IF NOT EXISTS started_by_source TEXT NOT NULL DEFAULT 'device';

		ALTER TABLE active.groups
			DROP CONSTRAINT IF EXISTS check_active_groups_started_by_source;

		ALTER TABLE active.groups
			ADD CONSTRAINT check_active_groups_started_by_source
			CHECK (started_by_source IN ('device', 'web'));

		CREATE INDEX IF NOT EXISTS idx_active_groups_started_by
			ON active.groups(started_by)
			WHERE started_by IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error adding started_by columns to active.groups: %w", err)
	}

	return tx.Commit()
}

func dropActiveGroupsStartedBy(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.37: Removing started_by columns from active.groups...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback active_groups_started_by migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS active.idx_active_groups_started_by;
		ALTER TABLE active.groups
			DROP CONSTRAINT IF EXISTS check_active_groups_started_by_source;
		ALTER TABLE active.groups DROP COLUMN IF EXISTS started_by_source;
		ALTER TABLE active.groups DROP COLUMN IF EXISTS started_by;
	`)
	if err != nil {
		return fmt.Errorf("error removing started_by columns from active.groups: %w", err)
	}

	return tx.Commit()
}
