package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	pauseStartedAtVersion     = "1.10.3"
	pauseStartedAtDescription = "Add pause_started_at column to work_sessions for break state persistence"
)

func init() {
	MigrationRegistry[pauseStartedAtVersion] = &Migration{
		Version:     pauseStartedAtVersion,
		Description: pauseStartedAtDescription,
		DependsOn:   []string{"1.10.1"}, // Depends on work_sessions table
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addPauseStartedAt(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropPauseStartedAt(ctx, db)
		},
	)
}

func addPauseStartedAt(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.10.3: Adding pause_started_at to active.work_sessions...")

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
		ALTER TABLE active.work_sessions
		ADD COLUMN IF NOT EXISTS pause_started_at TIMESTAMPTZ;
	`)
	if err != nil {
		return fmt.Errorf("error adding pause_started_at column: %w", err)
	}

	fmt.Println("Migration 1.10.3: Successfully added pause_started_at column")
	return tx.Commit()
}

func dropPauseStartedAt(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.10.3: Dropping pause_started_at column...")

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
		ALTER TABLE active.work_sessions
		DROP COLUMN IF EXISTS pause_started_at;
	`)
	if err != nil {
		return fmt.Errorf("error dropping pause_started_at column: %w", err)
	}

	fmt.Println("Migration 1.10.3: Successfully dropped pause_started_at column")
	return tx.Commit()
}
