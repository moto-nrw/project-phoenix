package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	platformOperatorMFAVersion     = "1.15.85"
	platformOperatorMFADescription = "Create platform.operator_mfa_credentials, _email_challenges, _recovery_codes, _trusted_devices (mirrors auth.mfa_* for moto-Operators)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     platformOperatorMFAVersion,
		Description: platformOperatorMFADescription,
		DependsOn:   []string{addMFALockoutToAccountsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return platformOperatorMFAUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return platformOperatorMFADown(ctx, db)
		},
	)
}

func platformOperatorMFAUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.85: Creating platform.operator_mfa_* tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in operator_mfa migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS platform.operator_mfa_credentials (
			id           BIGSERIAL PRIMARY KEY,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			operator_id  BIGINT NOT NULL,
			method       TEXT NOT NULL,
			enrolled_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_used_at TIMESTAMPTZ,
			CONSTRAINT fk_operator_mfa_credentials_operator
				FOREIGN KEY (operator_id) REFERENCES platform.operators(id) ON DELETE CASCADE,
			CONSTRAINT chk_operator_mfa_credentials_method CHECK (method IN ('email')),
			CONSTRAINT uniq_operator_mfa_credentials_op_method UNIQUE (operator_id, method)
		);
		CREATE INDEX IF NOT EXISTS idx_operator_mfa_credentials_operator_id
			ON platform.operator_mfa_credentials (operator_id);
		DROP TRIGGER IF EXISTS update_operator_mfa_credentials_updated_at ON platform.operator_mfa_credentials;
		CREATE TRIGGER update_operator_mfa_credentials_updated_at
		BEFORE UPDATE ON platform.operator_mfa_credentials
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		CREATE TABLE IF NOT EXISTS platform.operator_mfa_email_challenges (
			id          BIGSERIAL PRIMARY KEY,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			operator_id BIGINT NOT NULL,
			code_hash   TEXT NOT NULL,
			expires_at  TIMESTAMPTZ NOT NULL,
			consumed_at TIMESTAMPTZ,
			ip_address  INET,
			CONSTRAINT fk_operator_mfa_email_challenges_operator
				FOREIGN KEY (operator_id) REFERENCES platform.operators(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_operator_mfa_email_challenges_operator_active
			ON platform.operator_mfa_email_challenges (operator_id, expires_at DESC)
			WHERE consumed_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_operator_mfa_email_challenges_expires_at
			ON platform.operator_mfa_email_challenges (expires_at);
		DROP TRIGGER IF EXISTS update_operator_mfa_email_challenges_updated_at ON platform.operator_mfa_email_challenges;
		CREATE TRIGGER update_operator_mfa_email_challenges_updated_at
		BEFORE UPDATE ON platform.operator_mfa_email_challenges
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		CREATE TABLE IF NOT EXISTS platform.operator_mfa_recovery_codes (
			id          BIGSERIAL PRIMARY KEY,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			operator_id BIGINT NOT NULL,
			code_hash   TEXT NOT NULL,
			used_at     TIMESTAMPTZ,
			CONSTRAINT fk_operator_mfa_recovery_codes_operator
				FOREIGN KEY (operator_id) REFERENCES platform.operators(id) ON DELETE CASCADE,
			CONSTRAINT uniq_operator_mfa_recovery_codes_op_hash UNIQUE (operator_id, code_hash)
		);
		CREATE INDEX IF NOT EXISTS idx_operator_mfa_recovery_codes_operator_unused
			ON platform.operator_mfa_recovery_codes (operator_id)
			WHERE used_at IS NULL;
		DROP TRIGGER IF EXISTS update_operator_mfa_recovery_codes_updated_at ON platform.operator_mfa_recovery_codes;
		CREATE TRIGGER update_operator_mfa_recovery_codes_updated_at
		BEFORE UPDATE ON platform.operator_mfa_recovery_codes
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		CREATE TABLE IF NOT EXISTS platform.operator_mfa_trusted_devices (
			id           BIGSERIAL PRIMARY KEY,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			operator_id  BIGINT NOT NULL,
			token_hash   TEXT NOT NULL,
			user_agent   TEXT,
			ip_address   INET,
			expires_at   TIMESTAMPTZ NOT NULL,
			last_used_at TIMESTAMPTZ,
			revoked_at   TIMESTAMPTZ,
			CONSTRAINT fk_operator_mfa_trusted_devices_operator
				FOREIGN KEY (operator_id) REFERENCES platform.operators(id) ON DELETE CASCADE,
			CONSTRAINT uniq_operator_mfa_trusted_devices_op_token UNIQUE (operator_id, token_hash)
		);
		CREATE INDEX IF NOT EXISTS idx_operator_mfa_trusted_devices_operator_active
			ON platform.operator_mfa_trusted_devices (operator_id, expires_at DESC)
			WHERE revoked_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_operator_mfa_trusted_devices_expires_at
			ON platform.operator_mfa_trusted_devices (expires_at);
		DROP TRIGGER IF EXISTS update_operator_mfa_trusted_devices_updated_at ON platform.operator_mfa_trusted_devices;
		CREATE TRIGGER update_operator_mfa_trusted_devices_updated_at
		BEFORE UPDATE ON platform.operator_mfa_trusted_devices
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating platform.operator_mfa_* tables: %w", err)
	}

	return tx.Commit()
}

func platformOperatorMFADown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.85: Dropping platform.operator_mfa_* tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback in operator_mfa down migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_operator_mfa_trusted_devices_updated_at ON platform.operator_mfa_trusted_devices;
		DROP TRIGGER IF EXISTS update_operator_mfa_recovery_codes_updated_at  ON platform.operator_mfa_recovery_codes;
		DROP TRIGGER IF EXISTS update_operator_mfa_email_challenges_updated_at ON platform.operator_mfa_email_challenges;
		DROP TRIGGER IF EXISTS update_operator_mfa_credentials_updated_at      ON platform.operator_mfa_credentials;
		DROP TABLE IF EXISTS platform.operator_mfa_trusted_devices  CASCADE;
		DROP TABLE IF EXISTS platform.operator_mfa_recovery_codes   CASCADE;
		DROP TABLE IF EXISTS platform.operator_mfa_email_challenges CASCADE;
		DROP TABLE IF EXISTS platform.operator_mfa_credentials      CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping platform.operator_mfa_* tables: %w", err)
	}

	return tx.Commit()
}
