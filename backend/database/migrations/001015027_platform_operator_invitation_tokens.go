package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	platformOperatorInvitationTokensVersion     = "1.15.27"
	platformOperatorInvitationTokensDescription = "Create platform.operator_invitation_tokens table"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     platformOperatorInvitationTokensVersion,
		Description: platformOperatorInvitationTokensDescription,
		DependsOn:   []string{"1.15.26"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createPlatformOperatorInvitationTokensTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropPlatformOperatorInvitationTokensTable(ctx, db)
		},
	)
}

func createPlatformOperatorInvitationTokensTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.27: Creating platform.operator_invitation_tokens table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in operator invitation tokens migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS platform.operator_invitation_tokens (
			id                BIGSERIAL PRIMARY KEY,
			created_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			email             TEXT NOT NULL,
			token             TEXT NOT NULL,
			expires_at        TIMESTAMPTZ NOT NULL,
			used_at           TIMESTAMPTZ,
			created_by        BIGINT NOT NULL REFERENCES platform.operators(id) ON DELETE CASCADE,
			display_name      TEXT,
			email_sent_at     TIMESTAMPTZ,
			email_error       TEXT,
			email_retry_count INT NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating operator_invitation_tokens table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_operator_invitation_tokens_token
			ON platform.operator_invitation_tokens(token);
		CREATE INDEX IF NOT EXISTS idx_operator_invitation_tokens_email
			ON platform.operator_invitation_tokens(email);
		CREATE INDEX IF NOT EXISTS idx_operator_invitation_tokens_expires_at
			ON platform.operator_invitation_tokens(expires_at);
		CREATE INDEX IF NOT EXISTS idx_operator_invitation_tokens_created_by
			ON platform.operator_invitation_tokens(created_by);

		-- Guarantee at most one active (unused) invitation per email.
		CREATE UNIQUE INDEX IF NOT EXISTS idx_operator_invitation_tokens_one_active_per_email
			ON platform.operator_invitation_tokens(email)
			WHERE used_at IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for operator_invitation_tokens table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION platform.update_operator_invitation_token_updated_at()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		CREATE TRIGGER trg_operator_invitation_token_updated_at
			BEFORE UPDATE ON platform.operator_invitation_tokens
			FOR EACH ROW
			EXECUTE FUNCTION platform.update_operator_invitation_token_updated_at();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for operator_invitation_tokens: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		GRANT SELECT, INSERT, UPDATE, DELETE
			ON platform.operator_invitation_tokens TO phoenix_auth;
		GRANT USAGE, SELECT
			ON SEQUENCE platform.operator_invitation_tokens_id_seq TO phoenix_auth;
	`)
	if err != nil {
		return fmt.Errorf("error granting permissions on operator_invitation_tokens: %w", err)
	}

	return tx.Commit()
}

func dropPlatformOperatorInvitationTokensTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.27: Removing platform.operator_invitation_tokens table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in operator invitation tokens down migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS trg_operator_invitation_token_updated_at
			ON platform.operator_invitation_tokens;
		DROP FUNCTION IF EXISTS platform.update_operator_invitation_token_updated_at();
		DROP TABLE IF EXISTS platform.operator_invitation_tokens CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping platform.operator_invitation_tokens table: %w", err)
	}

	return tx.Commit()
}
