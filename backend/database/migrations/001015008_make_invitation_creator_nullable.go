package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	makeInvitationCreatorNullableVersion     = "1.15.8"
	makeInvitationCreatorNullableDescription = "Allow operator-created invitation tokens without an auth.accounts creator"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     makeInvitationCreatorNullableVersion,
		Description: makeInvitationCreatorNullableDescription,
		DependsOn:   []string{"1.15.7"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return makeInvitationCreatorNullable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackMakeInvitationCreatorNullable(ctx, db)
		},
	)
}

func makeInvitationCreatorNullable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.8: Allowing nullable invitation creators for operator-created invites...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			log.Printf("failed to rollback transaction in 1.15.8: %v", rbErr)
		}
	}()

	if _, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.invitation_tokens
		ALTER COLUMN created_by DROP NOT NULL;
	`); err != nil {
		return fmt.Errorf("failed to drop NOT NULL from auth.invitation_tokens.created_by: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.invitation_tokens
		DROP CONSTRAINT IF EXISTS fk_invitation_creator;
	`); err != nil {
		return fmt.Errorf("failed to drop invitation creator foreign key: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.invitation_tokens
		ADD CONSTRAINT fk_invitation_creator
		FOREIGN KEY (created_by) REFERENCES auth.accounts(id) ON DELETE SET NULL;
	`); err != nil {
		return fmt.Errorf("failed to recreate invitation creator foreign key: %w", err)
	}

	fmt.Println("Migration 1.15.8: Invitation creators can now be NULL")
	return tx.Commit()
}

func rollbackMakeInvitationCreatorNullable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.8: Restoring required invitation creators...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			log.Printf("failed to rollback transaction in 1.15.8 rollback: %v", rbErr)
		}
	}()

	var nullCreators int
	if err = tx.NewRaw(`
		SELECT COUNT(*)
		FROM auth.invitation_tokens
		WHERE created_by IS NULL
	`).Scan(ctx, &nullCreators); err != nil {
		return fmt.Errorf("failed to count invitation tokens without creators: %w", err)
	}
	if nullCreators > 0 {
		return fmt.Errorf("migration 1.15.8 cannot be rolled back while %d invitation tokens have NULL created_by", nullCreators)
	}

	if _, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.invitation_tokens
		DROP CONSTRAINT IF EXISTS fk_invitation_creator;
	`); err != nil {
		return fmt.Errorf("failed to drop nullable invitation creator foreign key: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.invitation_tokens
		ADD CONSTRAINT fk_invitation_creator
		FOREIGN KEY (created_by) REFERENCES auth.accounts(id) ON DELETE CASCADE;
	`); err != nil {
		return fmt.Errorf("failed to restore invitation creator foreign key: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.invitation_tokens
		ALTER COLUMN created_by SET NOT NULL;
	`); err != nil {
		return fmt.Errorf("failed to restore NOT NULL on auth.invitation_tokens.created_by: %w", err)
	}

	fmt.Println("Migration 1.15.8: Invitation creators are required again")
	return tx.Commit()
}
