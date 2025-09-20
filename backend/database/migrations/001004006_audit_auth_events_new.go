package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	AuditAuthEventsVersion     = "1.4.6"
	AuditAuthEventsDescription = "Create audit.auth_events table for authentication auditing"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AuditAuthEventsVersion] = &Migration{
		Version:     AuditAuthEventsVersion,
		Description: AuditAuthEventsDescription,
		DependsOn:   []string{"1.0.1"}, // Depends on auth.accounts
	}

	// Register the migration
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAuditAuthEventsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAuditAuthEventsTable(ctx, db)
		},
	)
}

// createAuditAuthEventsTable creates the audit.auth_events table
func createAuditAuthEventsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.6: Creating audit.auth_events table...")

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

	// Create the auth_events table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS audit.auth_events (
			id BIGSERIAL PRIMARY KEY,
			account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			event_type VARCHAR(50) NOT NULL,
			success BOOLEAN NOT NULL DEFAULT false,
			ip_address INET NOT NULL,
			user_agent TEXT,
			error_message TEXT,
			metadata JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating audit.auth_events table: %w", err)
	}

	// Create indexes for efficient querying
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_auth_events_account_id ON audit.auth_events(account_id);
		CREATE INDEX IF NOT EXISTS idx_auth_events_event_type ON audit.auth_events(event_type);
		CREATE INDEX IF NOT EXISTS idx_auth_events_created_at ON audit.auth_events(created_at);
		CREATE INDEX IF NOT EXISTS idx_auth_events_account_success ON audit.auth_events(account_id, success, created_at);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for audit.auth_events table: %w", err)
	}

	// Add comments
	_, err = tx.ExecContext(ctx, `
		COMMENT ON TABLE audit.auth_events IS 'Audit log of authentication events for security monitoring and compliance';
		COMMENT ON COLUMN audit.auth_events.event_type IS 'Type of auth event: login, logout, token_refresh, token_expired, password_reset, account_locked';
		COMMENT ON COLUMN audit.auth_events.metadata IS 'Additional context-specific data about the event';
	`)
	if err != nil {
		return fmt.Errorf("error adding comments to audit.auth_events table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropAuditAuthEventsTable drops the audit.auth_events table
func dropAuditAuthEventsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.6: Removing audit.auth_events table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in down migration: %v", err)
		}
	}()

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS audit.auth_events CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping audit.auth_events table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
