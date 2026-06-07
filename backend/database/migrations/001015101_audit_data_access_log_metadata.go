package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	auditDataAccessLogMetadataVersion     = "1.15.101"
	auditDataAccessLogMetadataDescription = "Add metadata jsonb to audit.data_access_log (matches auth_events/data_imports/data_deletions) so non-attendance access events — e.g. enrollment phase exports — can record event-specific context."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     auditDataAccessLogMetadataVersion,
		Description: auditDataAccessLogMetadataDescription,
		DependsOn:   []string{auditDataAccessLogVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addAuditDataAccessLogMetadata(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAuditDataAccessLogMetadata(ctx, db)
		},
	)
}

func addAuditDataAccessLogMetadata(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.101: Adding metadata column to audit.data_access_log...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Additive, backward-compatible: existing attendance-history rows
	// default to '{}'. Mirrors the metadata column already on
	// audit.auth_events and audit.data_imports (JSONB DEFAULT '{}',
	// nullable). No GRANT needed — INSERT/SELECT on the table already
	// cover all columns for phoenix_tenant.
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE audit.data_access_log
			ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}';

		COMMENT ON COLUMN audit.data_access_log.metadata IS
			'Event-specific context (e.g. enrollment phase export: phase_id, format, request_count, child_count)';
	`)
	if err != nil {
		return fmt.Errorf("error adding audit.data_access_log.metadata: %w", err)
	}
	fmt.Println("  ✓ audit.data_access_log.metadata — column added")

	return tx.Commit()
}

func dropAuditDataAccessLogMetadata(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.101: Dropping audit.data_access_log.metadata...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `ALTER TABLE audit.data_access_log DROP COLUMN IF EXISTS metadata;`)
	if err != nil {
		return fmt.Errorf("error dropping audit.data_access_log.metadata: %w", err)
	}

	return tx.Commit()
}
