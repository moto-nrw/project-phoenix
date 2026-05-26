package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	authMFARecoveryCodesVersion     = "1.15.80"
	authMFARecoveryCodesDescription = "Create auth.mfa_recovery_codes table (10 Argon2id-hashed single-use codes per account)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     authMFARecoveryCodesVersion,
		Description: authMFARecoveryCodesDescription,
		DependsOn:   []string{authMFACredentialsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return authMFARecoveryCodesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return authMFARecoveryCodesDown(ctx, db)
		},
	)
}

func authMFARecoveryCodesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.80: Creating auth.mfa_recovery_codes table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in mfa_recovery_codes migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.mfa_recovery_codes (
			id         BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			account_id BIGINT NOT NULL,
			code_hash  TEXT NOT NULL,
			used_at    TIMESTAMPTZ,
			CONSTRAINT fk_mfa_recovery_codes_account
				FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE,
			CONSTRAINT uniq_mfa_recovery_codes_account_hash UNIQUE (account_id, code_hash)
		);

		CREATE INDEX IF NOT EXISTS idx_mfa_recovery_codes_account_unused
			ON auth.mfa_recovery_codes (account_id)
			WHERE used_at IS NULL;

		DROP TRIGGER IF EXISTS update_mfa_recovery_codes_updated_at ON auth.mfa_recovery_codes;
		CREATE TRIGGER update_mfa_recovery_codes_updated_at
		BEFORE UPDATE ON auth.mfa_recovery_codes
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating auth.mfa_recovery_codes: %w", err)
	}

	return tx.Commit()
}

func authMFARecoveryCodesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.80: Dropping auth.mfa_recovery_codes...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback in mfa_recovery_codes down migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_mfa_recovery_codes_updated_at ON auth.mfa_recovery_codes;
		DROP TABLE IF EXISTS auth.mfa_recovery_codes CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.mfa_recovery_codes: %w", err)
	}

	return tx.Commit()
}
