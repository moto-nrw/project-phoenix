package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AddPositionToInvitationTokensVersion     = "1.6.17"
	AddPositionToInvitationTokensDescription = "Add position column to auth.invitation_tokens"
)

func init() {
	MigrationRegistry[AddPositionToInvitationTokensVersion] = &Migration{
		Version:     AddPositionToInvitationTokensVersion,
		Description: AddPositionToInvitationTokensDescription,
		DependsOn: []string{
			AuthInvitationTokensVersion, // 1.4.9
		},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addPositionToInvitationTokens(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removePositionFromInvitationTokens(ctx, db)
		},
	)
}

func addPositionToInvitationTokens(ctx context.Context, db *bun.DB) error {
	LogMigration(AddPositionToInvitationTokensVersion, "Adding position column to auth.invitation_tokens...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			logRollbackError(rbErr)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.invitation_tokens
		ADD COLUMN IF NOT EXISTS position TEXT;
	`)
	if err != nil {
		return fmt.Errorf("error adding position column to auth.invitation_tokens: %w", err)
	}

	return tx.Commit()
}

func removePositionFromInvitationTokens(ctx context.Context, db *bun.DB) error {
	LogMigration(AddPositionToInvitationTokensVersion, "Rolling back: Removing position column from auth.invitation_tokens...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			logRollbackError(rbErr)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.invitation_tokens
		DROP COLUMN IF EXISTS position;
	`)
	if err != nil {
		return fmt.Errorf("error removing position column from auth.invitation_tokens: %w", err)
	}

	return tx.Commit()
}
