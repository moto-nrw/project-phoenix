package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	platformOperatorEmailChangeTokensVersion     = "1.15.26"
	platformOperatorEmailChangeTokensDescription = "Create platform.operator_email_change_tokens table"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     platformOperatorEmailChangeTokensVersion,
		Description: platformOperatorEmailChangeTokensDescription,
		DependsOn:   []string{"1.15.25"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createPlatformOperatorEmailChangeTokensTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropPlatformOperatorEmailChangeTokensTable(ctx, db)
		},
	)
}

func createPlatformOperatorEmailChangeTokensTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.26: Creating platform.operator_email_change_tokens table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in operator email change tokens migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS platform.operator_email_change_tokens (
			id              BIGSERIAL PRIMARY KEY,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			operator_id     BIGINT NOT NULL REFERENCES platform.operators(id) ON DELETE CASCADE,
			new_email       TEXT NOT NULL,
			token           TEXT NOT NULL,
			expiry          TIMESTAMPTZ NOT NULL,
			used            BOOLEAN NOT NULL DEFAULT FALSE,
			email_sent_at   TIMESTAMPTZ,
			email_error     TEXT,
			email_retry_count INT NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating operator_email_change_tokens table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_email_change_tokens_token
			ON platform.operator_email_change_tokens(token);
		CREATE INDEX IF NOT EXISTS idx_email_change_tokens_operator_id
			ON platform.operator_email_change_tokens(operator_id);
		CREATE INDEX IF NOT EXISTS idx_email_change_tokens_expiry
			ON platform.operator_email_change_tokens(expiry);

		-- Guarantee at most one active (unused) token per operator.
		-- Concurrent transactions that both pass the application-level invalidation
		-- will conflict on this index, so the second INSERT rolls back.
		-- Note: expired-but-unused tokens remain in the index until the cleanup job
		-- marks or deletes them; this is harmless and avoids using CURRENT_TIMESTAMP
		-- which PostgreSQL rejects in partial-index predicates (not immutable).
		CREATE UNIQUE INDEX IF NOT EXISTS idx_email_change_tokens_one_active_per_operator
			ON platform.operator_email_change_tokens(operator_id)
			WHERE used = FALSE;
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for operator_email_change_tokens table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION platform.update_email_change_token_updated_at()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		CREATE TRIGGER trg_email_change_token_updated_at
			BEFORE UPDATE ON platform.operator_email_change_tokens
			FOR EACH ROW
			EXECUTE FUNCTION platform.update_email_change_token_updated_at();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for operator_email_change_tokens: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		GRANT SELECT, INSERT, UPDATE, DELETE
			ON platform.operator_email_change_tokens TO phoenix_auth;
		GRANT USAGE, SELECT
			ON SEQUENCE platform.operator_email_change_tokens_id_seq TO phoenix_auth;
	`)
	if err != nil {
		return fmt.Errorf("error granting permissions on operator_email_change_tokens: %w", err)
	}

	return tx.Commit()
}

func dropPlatformOperatorEmailChangeTokensTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.26: Removing platform.operator_email_change_tokens table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in operator email change tokens down migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS trg_email_change_token_updated_at
			ON platform.operator_email_change_tokens;
		DROP FUNCTION IF EXISTS platform.update_email_change_token_updated_at();
		DROP TABLE IF EXISTS platform.operator_email_change_tokens CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping platform.operator_email_change_tokens table: %w", err)
	}

	return tx.Commit()
}
