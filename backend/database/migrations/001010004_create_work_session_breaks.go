package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	workSessionBreaksVersion     = "1.10.4"
	workSessionBreaksDescription = "Create work_session_breaks table and drop pause_started_at column"
)

func init() {
	MigrationRegistry[workSessionBreaksVersion] = &Migration{
		Version:     workSessionBreaksVersion,
		Description: workSessionBreaksDescription,
		DependsOn:   []string{"1.10.3"}, // Depends on pause_started_at migration
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createWorkSessionBreaks(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropWorkSessionBreaks(ctx, db)
		},
	)
}

func createWorkSessionBreaks(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.10.4: Creating active.work_session_breaks table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS active.work_session_breaks (
			id               BIGSERIAL PRIMARY KEY,
			session_id       BIGINT NOT NULL REFERENCES active.work_sessions(id) ON DELETE CASCADE,
			started_at       TIMESTAMPTZ NOT NULL,
			ended_at         TIMESTAMPTZ,
			duration_minutes INT NOT NULL DEFAULT 0,
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_wsb_session_id ON active.work_session_breaks(session_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating work_session_breaks table: %w", err)
	}

	// Drop pause_started_at column (replaced by break rows)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.work_sessions DROP COLUMN IF EXISTS pause_started_at;
	`)
	if err != nil {
		return fmt.Errorf("error dropping pause_started_at column: %w", err)
	}

	fmt.Println("Migration 1.10.4: Successfully created work_session_breaks table and dropped pause_started_at")
	return tx.Commit()
}

func dropWorkSessionBreaks(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.10.4: Dropping work_session_breaks table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Re-add pause_started_at column
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.work_sessions
		ADD COLUMN IF NOT EXISTS pause_started_at TIMESTAMPTZ;
	`)
	if err != nil {
		return fmt.Errorf("error re-adding pause_started_at column: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS active.work_session_breaks CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping work_session_breaks table: %w", err)
	}

	fmt.Println("Migration 1.10.4: Successfully rolled back")
	return tx.Commit()
}
