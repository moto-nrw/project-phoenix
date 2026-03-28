package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	disableLegacyBootstrapAdminVersion     = "1.15.7"
	disableLegacyBootstrapAdminDescription = "Disable legacy bootstrap admin account while keeping the row for historical references"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     disableLegacyBootstrapAdminVersion,
		Description: disableLegacyBootstrapAdminDescription,
		DependsOn:   []string{"1.15.6"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return disableLegacyBootstrapAdmin(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackDisableLegacyBootstrapAdmin(ctx, db)
		},
	)
}

func disableLegacyBootstrapAdmin(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.7: Disabling legacy bootstrap admin account...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			log.Printf("failed to rollback transaction in 1.15.7: %v", rbErr)
		}
	}()

	var bootstrapAccountID int64
	err = tx.NewRaw(`
		SELECT id
		FROM auth.accounts
		WHERE username = ?
	`, "admin").Scan(ctx, &bootstrapAccountID)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("Migration 1.15.7: Legacy bootstrap admin account not present, skipping")
			return tx.Commit()
		}
		return fmt.Errorf("failed to verify bootstrap admin account: %w", err)
	}
	if bootstrapAccountID <= 0 {
		fmt.Println("Migration 1.15.7: Legacy bootstrap admin account not present, skipping")
		return tx.Commit()
	}

	if _, err = tx.ExecContext(ctx, `
		UPDATE auth.accounts
		SET active = false,
			password_hash = NULL,
			pin_hash = NULL,
			pin_attempts = 0,
			pin_locked_until = NULL
		WHERE id = ?;
	`, bootstrapAccountID); err != nil {
		return fmt.Errorf("failed to deactivate bootstrap admin account: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM auth.tokens WHERE account_id = ?;`, bootstrapAccountID); err != nil {
		return fmt.Errorf("failed to delete bootstrap admin tokens: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM auth.account_roles WHERE account_id = ?;`, bootstrapAccountID); err != nil {
		return fmt.Errorf("failed to delete bootstrap admin roles: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM auth.account_permissions WHERE account_id = ?;`, bootstrapAccountID); err != nil {
		return fmt.Errorf("failed to delete bootstrap admin permissions: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?;`, bootstrapAccountID); err != nil {
		return fmt.Errorf("failed to delete bootstrap admin tenant mappings: %w", err)
	}

	fmt.Println("Migration 1.15.7: Legacy bootstrap admin account disabled")
	return tx.Commit()
}

func rollbackDisableLegacyBootstrapAdmin(ctx context.Context, db *bun.DB) error {
	_ = ctx
	_ = db
	return fmt.Errorf("migration 1.15.7 is irreversible: bootstrap admin credentials, roles, permissions, tokens, and tenant mappings were removed")
}
