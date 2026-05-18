package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	authMFACredentialsVersion     = "1.15.60"
	authMFACredentialsDescription = "Create auth.mfa_credentials table (per-account MFA enrollment, v1: email-only)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     authMFACredentialsVersion,
		Description: authMFACredentialsDescription,
		DependsOn:   []string{"1.0.1"}, // auth.accounts
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return authMFACredentialsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return authMFACredentialsDown(ctx, db)
		},
	)
}

func authMFACredentialsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.60: Creating auth.mfa_credentials table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in mfa_credentials migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.mfa_credentials (
			id           BIGSERIAL PRIMARY KEY,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			account_id   BIGINT NOT NULL,
			method       TEXT NOT NULL,
			enrolled_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_used_at TIMESTAMPTZ,
			CONSTRAINT fk_mfa_credentials_account
				FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE,
			CONSTRAINT chk_mfa_credentials_method CHECK (method IN ('email')),
			CONSTRAINT uniq_mfa_credentials_account_method UNIQUE (account_id, method)
		);

		CREATE INDEX IF NOT EXISTS idx_mfa_credentials_account_id
			ON auth.mfa_credentials (account_id);

		DROP TRIGGER IF EXISTS update_mfa_credentials_updated_at ON auth.mfa_credentials;
		CREATE TRIGGER update_mfa_credentials_updated_at
		BEFORE UPDATE ON auth.mfa_credentials
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating auth.mfa_credentials: %w", err)
	}

	return tx.Commit()
}

func authMFACredentialsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.60: Dropping auth.mfa_credentials...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback in mfa_credentials down migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_mfa_credentials_updated_at ON auth.mfa_credentials;
		DROP TABLE IF EXISTS auth.mfa_credentials CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.mfa_credentials: %w", err)
	}

	return tx.Commit()
}
