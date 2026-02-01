package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	AuditDataImportsVersion     = "1.6.23"
	AuditDataImportsDescription = "Create audit.data_imports table for import history tracking"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AuditDataImportsVersion] = &Migration{
		Version:     AuditDataImportsVersion,
		Description: AuditDataImportsDescription,
		DependsOn:   []string{"1.0.1"}, // Depends on auth.accounts
	}

	// Register the migration
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAuditDataImportsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAuditDataImportsTable(ctx, db)
		},
	)
}

// createAuditDataImportsTable creates the audit.data_imports table
func createAuditDataImportsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.17: Creating audit.data_imports table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in up migration: %v", err)
		}
	}()

	// Create the data_imports table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS audit.data_imports (
			id BIGSERIAL PRIMARY KEY,
			entity_type TEXT NOT NULL,
			filename TEXT NOT NULL,
			total_rows INT NOT NULL,
			created_count INT NOT NULL DEFAULT 0,
			updated_count INT NOT NULL DEFAULT 0,
			skipped_count INT NOT NULL DEFAULT 0,
			error_count INT NOT NULL DEFAULT 0,
			warning_count INT NOT NULL DEFAULT 0,
			dry_run BOOLEAN NOT NULL DEFAULT FALSE,
			imported_by BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			started_at TIMESTAMPTZ NOT NULL,
			completed_at TIMESTAMPTZ,
			metadata JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating audit.data_imports table: %w", err)
	}

	// Create indexes for efficient querying
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_data_imports_entity_type ON audit.data_imports(entity_type);
		CREATE INDEX IF NOT EXISTS idx_data_imports_imported_by ON audit.data_imports(imported_by);
		CREATE INDEX IF NOT EXISTS idx_data_imports_created_at ON audit.data_imports(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_data_imports_dry_run ON audit.data_imports(dry_run, created_at DESC);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for audit.data_imports table: %w", err)
	}

	// Add comments for documentation
	_, err = tx.ExecContext(ctx, `
		COMMENT ON TABLE audit.data_imports IS 'Import history tracking for CSV/Excel imports';
		COMMENT ON COLUMN audit.data_imports.entity_type IS 'Type of entity imported (student, teacher, room, etc.)';
		COMMENT ON COLUMN audit.data_imports.filename IS 'Original filename of imported file';
		COMMENT ON COLUMN audit.data_imports.total_rows IS 'Total number of rows in the import file';
		COMMENT ON COLUMN audit.data_imports.created_count IS 'Number of new records created';
		COMMENT ON COLUMN audit.data_imports.updated_count IS 'Number of existing records updated';
		COMMENT ON COLUMN audit.data_imports.skipped_count IS 'Number of rows skipped';
		COMMENT ON COLUMN audit.data_imports.error_count IS 'Number of rows with errors';
		COMMENT ON COLUMN audit.data_imports.warning_count IS 'Number of rows with warnings';
		COMMENT ON COLUMN audit.data_imports.dry_run IS 'Whether this was a preview (dry run) or actual import';
		COMMENT ON COLUMN audit.data_imports.imported_by IS 'Account ID of user who performed the import';
		COMMENT ON COLUMN audit.data_imports.started_at IS 'When the import started';
		COMMENT ON COLUMN audit.data_imports.completed_at IS 'When the import completed';
		COMMENT ON COLUMN audit.data_imports.metadata IS 'Additional import metadata (error details, bulk actions, etc.)';
	`)
	if err != nil {
		return fmt.Errorf("error adding comments to audit.data_imports table: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Println("✓ Migration 1.6.17: audit.data_imports table created successfully")
	return nil
}

// dropAuditDataImportsTable drops the audit.data_imports table (rollback)
func dropAuditDataImportsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.17: Rolling back audit.data_imports table...")

	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS audit.data_imports CASCADE`)
	if err != nil {
		return fmt.Errorf("error dropping audit.data_imports table: %w", err)
	}

	log.Println("✓ Migration 1.6.17: audit.data_imports table dropped successfully")
	return nil
}
