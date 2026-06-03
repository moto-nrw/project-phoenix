package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	addMFALockoutToAccountsVersion     = "1.15.84"
	addMFALockoutToAccountsDescription = "Add mfa_attempts + mfa_locked_until columns to auth.accounts and platform.operators (mirrors PIN-Lockout pattern)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addMFALockoutToAccountsVersion,
		Description: addMFALockoutToAccountsDescription,
		DependsOn: []string{
			"1.0.1", // auth.accounts
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addMFALockoutToAccountsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addMFALockoutToAccountsDown(ctx, db)
		},
	)
}

func addMFALockoutToAccountsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.84: Adding mfa_attempts + mfa_locked_until to auth.accounts and platform.operators...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in add_mfa_lockout_to_accounts migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.accounts
			ADD COLUMN IF NOT EXISTS mfa_attempts     INT NOT NULL DEFAULT 0,
			ADD COLUMN IF NOT EXISTS mfa_locked_until TIMESTAMPTZ;

		ALTER TABLE platform.operators
			ADD COLUMN IF NOT EXISTS mfa_attempts     INT NOT NULL DEFAULT 0,
			ADD COLUMN IF NOT EXISTS mfa_locked_until TIMESTAMPTZ;
	`)
	if err != nil {
		return fmt.Errorf("error adding mfa lockout columns: %w", err)
	}

	return tx.Commit()
}

func addMFALockoutToAccountsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.84: Dropping mfa_attempts + mfa_locked_until from auth.accounts and platform.operators...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback in add_mfa_lockout_to_accounts down migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.accounts
			DROP COLUMN IF EXISTS mfa_attempts,
			DROP COLUMN IF EXISTS mfa_locked_until;

		ALTER TABLE platform.operators
			DROP COLUMN IF EXISTS mfa_attempts,
			DROP COLUMN IF EXISTS mfa_locked_until;
	`)
	if err != nil {
		return fmt.Errorf("error dropping mfa lockout columns: %w", err)
	}

	return tx.Commit()
}
