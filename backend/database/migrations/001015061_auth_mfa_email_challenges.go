package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	authMFAEmailChallengesVersion     = "1.15.61"
	authMFAEmailChallengesDescription = "Create auth.mfa_email_challenges table (active 6-digit email codes, hashed, time-limited, single-use)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     authMFAEmailChallengesVersion,
		Description: authMFAEmailChallengesDescription,
		DependsOn:   []string{authMFACredentialsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return authMFAEmailChallengesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return authMFAEmailChallengesDown(ctx, db)
		},
	)
}

func authMFAEmailChallengesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.61: Creating auth.mfa_email_challenges table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in mfa_email_challenges migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.mfa_email_challenges (
			id          BIGSERIAL PRIMARY KEY,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			account_id  BIGINT NOT NULL,
			code_hash   TEXT NOT NULL,
			expires_at  TIMESTAMPTZ NOT NULL,
			consumed_at TIMESTAMPTZ,
			ip_address  INET,
			CONSTRAINT fk_mfa_email_challenges_account
				FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_mfa_email_challenges_account_active
			ON auth.mfa_email_challenges (account_id, expires_at DESC)
			WHERE consumed_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_mfa_email_challenges_expires_at
			ON auth.mfa_email_challenges (expires_at);

		DROP TRIGGER IF EXISTS update_mfa_email_challenges_updated_at ON auth.mfa_email_challenges;
		CREATE TRIGGER update_mfa_email_challenges_updated_at
		BEFORE UPDATE ON auth.mfa_email_challenges
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating auth.mfa_email_challenges: %w", err)
	}

	return tx.Commit()
}

func authMFAEmailChallengesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.61: Dropping auth.mfa_email_challenges...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback in mfa_email_challenges down migration: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_mfa_email_challenges_updated_at ON auth.mfa_email_challenges;
		DROP TABLE IF EXISTS auth.mfa_email_challenges CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.mfa_email_challenges: %w", err)
	}

	return tx.Commit()
}
