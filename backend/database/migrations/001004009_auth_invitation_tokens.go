package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	AuthInvitationTokensVersion     = "1.4.9"
	AuthInvitationTokensDescription = "Create auth.invitation_tokens table"
)

func init() {
	MigrationRegistry[AuthInvitationTokensVersion] = &Migration{
		Version:     AuthInvitationTokensVersion,
		Description: AuthInvitationTokensDescription,
		DependsOn:   []string{"1.0.4", "1.0.1"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAuthInvitationTokensTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAuthInvitationTokensTable(ctx, db)
		},
	)
}

func createAuthInvitationTokensTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.9: Creating auth.invitation_tokens table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in invitation tokens migration: %v", rbErr)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.invitation_tokens (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			email TEXT NOT NULL,
			token TEXT NOT NULL,
			role_id BIGINT NOT NULL,
			created_by BIGINT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			first_name TEXT,
			last_name TEXT,
			CONSTRAINT fk_invitation_role FOREIGN KEY (role_id) REFERENCES auth.roles(id) ON DELETE RESTRICT,
			CONSTRAINT fk_invitation_creator FOREIGN KEY (created_by) REFERENCES auth.accounts(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating auth.invitation_tokens table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_invitation_tokens_token ON auth.invitation_tokens(token);
		CREATE INDEX IF NOT EXISTS idx_invitation_tokens_email ON auth.invitation_tokens(email);
		CREATE INDEX IF NOT EXISTS idx_invitation_tokens_expires_at ON auth.invitation_tokens(expires_at);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for auth.invitation_tokens: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_auth_invitation_tokens_updated_at ON auth.invitation_tokens;
		CREATE TRIGGER update_auth_invitation_tokens_updated_at
		BEFORE UPDATE ON auth.invitation_tokens
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for auth.invitation_tokens: %w", err)
	}

	return tx.Commit()
}

func dropAuthInvitationTokensTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.9: Dropping auth.invitation_tokens table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in invitation tokens down migration: %v", rbErr)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_auth_invitation_tokens_updated_at ON auth.invitation_tokens;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for auth.invitation_tokens: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS auth.invitation_tokens CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.invitation_tokens table: %w", err)
	}

	return tx.Commit()
}
