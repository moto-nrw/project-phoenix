package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AuthEmailDeliveryColumnsVersion     = "1.5.0"
	AuthEmailDeliveryColumnsDescription = "Add email delivery tracking columns to auth tables"
)

func init() {
	MigrationRegistry[AuthEmailDeliveryColumnsVersion] = &Migration{
		Version:     AuthEmailDeliveryColumnsVersion,
		Description: AuthEmailDeliveryColumnsDescription,
		DependsOn: []string{
			AuthInvitationTokensVersion,
			AuthPasswordResetTokensVersion,
		},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return applyAuthEmailDeliveryColumns(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAuthEmailDeliveryColumns(ctx, db)
		},
	)
}

func applyAuthEmailDeliveryColumns(ctx context.Context, db *bun.DB) error {
	LogMigration(AuthEmailDeliveryColumnsVersion, "Adding email delivery tracking columns to auth tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			logRollbackError(rbErr)
		}
	}()

	if _, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.invitation_tokens
		ADD COLUMN IF NOT EXISTS email_sent_at TIMESTAMPTZ,
		ADD COLUMN IF NOT EXISTS email_error TEXT,
		ADD COLUMN IF NOT EXISTS email_retry_count INTEGER NOT NULL DEFAULT 0;
	`); err != nil {
		return fmt.Errorf("failed altering auth.invitation_tokens for email delivery columns: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.password_reset_tokens
		ADD COLUMN IF NOT EXISTS email_sent_at TIMESTAMPTZ,
		ADD COLUMN IF NOT EXISTS email_error TEXT,
		ADD COLUMN IF NOT EXISTS email_retry_count INTEGER NOT NULL DEFAULT 0;
	`); err != nil {
		return fmt.Errorf("failed altering auth.password_reset_tokens for email delivery columns: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit email delivery migration: %w", err)
	}
	return nil
}

func dropAuthEmailDeliveryColumns(ctx context.Context, db *bun.DB) error {
	LogMigration(AuthEmailDeliveryColumnsVersion, "Rolling back: Removing email delivery tracking columns from auth tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			logRollbackError(rbErr)
		}
	}()

	if _, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.invitation_tokens
		DROP COLUMN IF EXISTS email_sent_at,
		DROP COLUMN IF EXISTS email_error,
		DROP COLUMN IF EXISTS email_retry_count;
	`); err != nil {
		return fmt.Errorf("failed dropping columns from auth.invitation_tokens: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.password_reset_tokens
		DROP COLUMN IF EXISTS email_sent_at,
		DROP COLUMN IF EXISTS email_error,
		DROP COLUMN IF EXISTS email_retry_count;
	`); err != nil {
		return fmt.Errorf("failed dropping columns from auth.password_reset_tokens: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit email delivery down migration: %w", err)
	}
	return nil
}
