package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	passkeyCredentialsVersion     = "1.15.133"
	passkeyCredentialsDescription = "Add WebAuthn passkey credentials and ceremony sessions"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     passkeyCredentialsVersion,
		Description: passkeyCredentialsDescription,
		DependsOn:   []string{unregisteredTagScansVersion, guardianFKRestrictVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return passkeyCredentialsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return passkeyCredentialsDown(ctx, db)
		},
	)
}

func passkeyCredentialsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.133: Creating passkey credential tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback passkey credential migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.passkey_credentials (
			id              BIGSERIAL PRIMARY KEY,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			account_id      BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			user_handle     BYTEA NOT NULL,
			credential_id   BYTEA NOT NULL,
			credential_json JSONB NOT NULL,
			name            TEXT NOT NULL DEFAULT '',
			last_used_at    TIMESTAMPTZ,
			revoked_at      TIMESTAMPTZ,
			CONSTRAINT uniq_passkey_credentials_credential_id UNIQUE (credential_id)
		);

		CREATE INDEX IF NOT EXISTS idx_passkey_credentials_account_active
			ON auth.passkey_credentials (account_id, created_at ASC)
			WHERE revoked_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_passkey_credentials_user_handle_active
			ON auth.passkey_credentials (user_handle)
			WHERE revoked_at IS NULL;

		DROP TRIGGER IF EXISTS update_passkey_credentials_updated_at ON auth.passkey_credentials;
		CREATE TRIGGER update_passkey_credentials_updated_at
		BEFORE UPDATE ON auth.passkey_credentials
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		CREATE TABLE IF NOT EXISTS auth.passkey_sessions (
			id              TEXT PRIMARY KEY,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			account_id      BIGINT REFERENCES auth.accounts(id) ON DELETE CASCADE,
			tenant_id       BIGINT REFERENCES platform.schools(id) ON DELETE CASCADE,
			purpose         TEXT NOT NULL CHECK (purpose IN ('registration', 'login')),
			rp_id           TEXT NOT NULL,
			expected_origin TEXT NOT NULL,
			session_json    JSONB NOT NULL,
			expires_at      TIMESTAMPTZ NOT NULL,
			consumed_at     TIMESTAMPTZ
		);

		CREATE INDEX IF NOT EXISTS idx_passkey_sessions_expiry
			ON auth.passkey_sessions (expires_at);
		CREATE INDEX IF NOT EXISTS idx_passkey_sessions_account
			ON auth.passkey_sessions (account_id, created_at DESC)
			WHERE account_id IS NOT NULL;

		DROP TRIGGER IF EXISTS update_passkey_sessions_updated_at ON auth.passkey_sessions;
		CREATE TRIGGER update_passkey_sessions_updated_at
		BEFORE UPDATE ON auth.passkey_sessions
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		CREATE TABLE IF NOT EXISTS platform.operator_passkey_credentials (
			id              BIGSERIAL PRIMARY KEY,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			operator_id     BIGINT NOT NULL REFERENCES platform.operators(id) ON DELETE CASCADE,
			user_handle     BYTEA NOT NULL,
			credential_id   BYTEA NOT NULL,
			credential_json JSONB NOT NULL,
			name            TEXT NOT NULL DEFAULT '',
			last_used_at    TIMESTAMPTZ,
			revoked_at      TIMESTAMPTZ,
			CONSTRAINT uniq_operator_passkey_credentials_credential_id UNIQUE (credential_id)
		);

		CREATE INDEX IF NOT EXISTS idx_operator_passkey_credentials_operator_active
			ON platform.operator_passkey_credentials (operator_id, created_at ASC)
			WHERE revoked_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_operator_passkey_credentials_user_handle_active
			ON platform.operator_passkey_credentials (user_handle)
			WHERE revoked_at IS NULL;

		DROP TRIGGER IF EXISTS update_operator_passkey_credentials_updated_at ON platform.operator_passkey_credentials;
		CREATE TRIGGER update_operator_passkey_credentials_updated_at
		BEFORE UPDATE ON platform.operator_passkey_credentials
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		CREATE TABLE IF NOT EXISTS platform.operator_passkey_sessions (
			id              TEXT PRIMARY KEY,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			operator_id     BIGINT REFERENCES platform.operators(id) ON DELETE CASCADE,
			purpose         TEXT NOT NULL CHECK (purpose IN ('registration', 'login')),
			rp_id           TEXT NOT NULL,
			expected_origin TEXT NOT NULL,
			session_json    JSONB NOT NULL,
			expires_at      TIMESTAMPTZ NOT NULL,
			consumed_at     TIMESTAMPTZ
		);

		CREATE INDEX IF NOT EXISTS idx_operator_passkey_sessions_expiry
			ON platform.operator_passkey_sessions (expires_at);
		CREATE INDEX IF NOT EXISTS idx_operator_passkey_sessions_operator
			ON platform.operator_passkey_sessions (operator_id, created_at DESC)
			WHERE operator_id IS NOT NULL;

		DROP TRIGGER IF EXISTS update_operator_passkey_sessions_updated_at ON platform.operator_passkey_sessions;
		CREATE TRIGGER update_operator_passkey_sessions_updated_at
		BEFORE UPDATE ON platform.operator_passkey_sessions
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		GRANT SELECT, INSERT, UPDATE, DELETE ON auth.passkey_credentials TO phoenix_auth;
		GRANT SELECT, INSERT, UPDATE, DELETE ON auth.passkey_sessions TO phoenix_auth;
		GRANT USAGE, SELECT ON SEQUENCE auth.passkey_credentials_id_seq TO phoenix_auth;
		GRANT SELECT, INSERT, UPDATE, DELETE ON platform.operator_passkey_credentials TO phoenix_auth;
		GRANT SELECT, INSERT, UPDATE, DELETE ON platform.operator_passkey_sessions TO phoenix_auth;
		GRANT USAGE, SELECT ON SEQUENCE platform.operator_passkey_credentials_id_seq TO phoenix_auth;
	`)
	if err != nil {
		return fmt.Errorf("error creating passkey credential tables: %w", err)
	}

	return tx.Commit()
}

func passkeyCredentialsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.133: Dropping passkey credential tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback passkey credential down migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS platform.operator_passkey_sessions CASCADE;
		DROP TABLE IF EXISTS platform.operator_passkey_credentials CASCADE;
		DROP TABLE IF EXISTS auth.passkey_sessions CASCADE;
		DROP TABLE IF EXISTS auth.passkey_credentials CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping passkey credential tables: %w", err)
	}

	return tx.Commit()
}
