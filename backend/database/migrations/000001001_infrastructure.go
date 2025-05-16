package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	InfrastructureVersion     = "0.1.1"
	InfrastructureDescription = "Initial infrastructure setup"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[InfrastructureVersion] = &Migration{
		Version:     InfrastructureVersion,
		Description: InfrastructureDescription,
		DependsOn:   []string{"0.0.0", "0.1.0"}, // Depends on schemas and core functions
	}

	// Migration 1.0.0: Initial infrastructure setup
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.0.0: Setting up migration infrastructure...")

			// Begin a transaction for atomicity
			tx, err := db.BeginTx(ctx, &sql.TxOptions{})
			if err != nil {
				return fmt.Errorf("failed to begin transaction: %w", err)
			}
			defer func() { _ = tx.Rollback() }()

			// Create the migration_metadata table in the meta schema
			_, err = tx.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS meta.migration_metadata (
					version VARCHAR(50) PRIMARY KEY,
					description TEXT NOT NULL,
					applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					execution_time_ms INTEGER NOT NULL,
					checksum TEXT NOT NULL,
					success BOOLEAN NOT NULL,
					error_message TEXT,
					applied_by TEXT NOT NULL
				)
			`)
			if err != nil {
				return fmt.Errorf("error creating migration_metadata table: %w", err)
			}

			// Create index on applied_at for query performance
			_, err = tx.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_migration_metadata_applied_at 
				ON meta.migration_metadata(applied_at)
			`)
			if err != nil {
				return fmt.Errorf("error creating index on migration_metadata: %w", err)
			}

			// Add any PostgreSQL extensions needed
			// For example, uuid-ossp for UUID generation
			_, err = tx.ExecContext(ctx, `
				CREATE EXTENSION IF NOT EXISTS "uuid-ossp"
			`)
			if err != nil {
				return fmt.Errorf("error creating uuid-ossp extension: %w", err)
			}

			// Commit the transaction
			return tx.Commit()
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.0.0: Removing infrastructure setup...")

			// Begin a transaction for atomicity
			tx, err := db.BeginTx(ctx, &sql.TxOptions{})
			if err != nil {
				return fmt.Errorf("failed to begin transaction: %w", err)
			}
			defer func() { _ = tx.Rollback() }()

			// Drop the migration_metadata table in meta schema
			_, err = tx.ExecContext(ctx, `
				DROP TABLE IF EXISTS meta.migration_metadata
			`)
			if err != nil {
				return fmt.Errorf("error dropping migration_metadata table: %w", err)
			}

			// Commit the transaction
			return tx.Commit()
		},
	)
}
