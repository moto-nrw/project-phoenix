package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	authMFATrustedDevicesVersion     = "1.15.83"
	authMFATrustedDevicesDescription = "Create auth.mfa_trusted_devices table (HMAC-signed device tokens, opaque hashes stored, expiring)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     authMFATrustedDevicesVersion,
		Description: authMFATrustedDevicesDescription,
		DependsOn:   []string{authMFACredentialsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return authMFATrustedDevicesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return authMFATrustedDevicesDown(ctx, db)
		},
	)
}

func authMFATrustedDevicesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.83: Creating auth.mfa_trusted_devices table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in mfa_trusted_devices migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.mfa_trusted_devices (
			id           BIGSERIAL PRIMARY KEY,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			account_id   BIGINT NOT NULL,
			token_hash   TEXT NOT NULL,
			user_agent   TEXT,
			ip_address   INET,
			expires_at   TIMESTAMPTZ NOT NULL,
			last_used_at TIMESTAMPTZ,
			revoked_at   TIMESTAMPTZ,
			CONSTRAINT fk_mfa_trusted_devices_account
				FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE,
			CONSTRAINT uniq_mfa_trusted_devices_account_token UNIQUE (account_id, token_hash)
		);

		CREATE INDEX IF NOT EXISTS idx_mfa_trusted_devices_account_active
			ON auth.mfa_trusted_devices (account_id, expires_at DESC)
			WHERE revoked_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_mfa_trusted_devices_expires_at
			ON auth.mfa_trusted_devices (expires_at);

		DROP TRIGGER IF EXISTS update_mfa_trusted_devices_updated_at ON auth.mfa_trusted_devices;
		CREATE TRIGGER update_mfa_trusted_devices_updated_at
		BEFORE UPDATE ON auth.mfa_trusted_devices
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating auth.mfa_trusted_devices: %w", err)
	}

	return tx.Commit()
}

func authMFATrustedDevicesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.83: Dropping auth.mfa_trusted_devices...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback in mfa_trusted_devices down migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_mfa_trusted_devices_updated_at ON auth.mfa_trusted_devices;
		DROP TABLE IF EXISTS auth.mfa_trusted_devices CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.mfa_trusted_devices: %w", err)
	}

	return tx.Commit()
}
