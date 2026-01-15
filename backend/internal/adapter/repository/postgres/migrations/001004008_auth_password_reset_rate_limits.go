package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AuthPasswordResetRateLimitsVersion     = "1.4.8"
	AuthPasswordResetRateLimitsDescription = "Create auth.password_reset_rate_limits table for per-email password reset throttling"
)

func init() {
	MigrationRegistry[AuthPasswordResetRateLimitsVersion] = &Migration{
		Version:     AuthPasswordResetRateLimitsVersion,
		Description: AuthPasswordResetRateLimitsDescription,
		DependsOn: []string{
			AuthPasswordResetTokensVersion,
		},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAuthPasswordResetRateLimitsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAuthPasswordResetRateLimitsTable(ctx, db)
		},
	)
}

func createAuthPasswordResetRateLimitsTable(ctx context.Context, db *bun.DB) error {
	LogMigration(AuthPasswordResetRateLimitsVersion, "Creating auth.password_reset_rate_limits table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			logRollbackError(rbErr)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.password_reset_rate_limits (
			email TEXT PRIMARY KEY,
			attempts INT NOT NULL DEFAULT 1,
			window_start TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating auth.password_reset_rate_limits table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_password_reset_rate_limits_window_start
			ON auth.password_reset_rate_limits(window_start);
	`)
	if err != nil {
		return fmt.Errorf("error creating index for auth.password_reset_rate_limits: %w", err)
	}

	return tx.Commit()
}

func dropAuthPasswordResetRateLimitsTable(ctx context.Context, db *bun.DB) error {
	LogMigration(AuthPasswordResetRateLimitsVersion, "Rolling back: Dropping auth.password_reset_rate_limits table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			logRollbackError(rbErr)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS idx_password_reset_rate_limits_window_start;
	`)
	if err != nil {
		return fmt.Errorf("error dropping index for auth.password_reset_rate_limits: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS auth.password_reset_rate_limits CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.password_reset_rate_limits table: %w", err)
	}

	return tx.Commit()
}
