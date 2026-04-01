package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addGlobalAccountAvatarVersion     = "1.15.24"
	addGlobalAccountAvatarDescription = "Add global avatar to auth.accounts and backfill from tenant profiles"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addGlobalAccountAvatarVersion,
		Description: addGlobalAccountAvatarDescription,
		DependsOn:   []string{"1.15.23"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addGlobalAccountAvatarUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addGlobalAccountAvatarDown(ctx, db)
		},
	)
}

func addGlobalAccountAvatarUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.24: Adding global avatar to auth.accounts...")

	if _, err := db.ExecContext(ctx, `
		ALTER TABLE auth.accounts
		ADD COLUMN IF NOT EXISTS avatar TEXT;
	`); err != nil {
		return fmt.Errorf("add auth.accounts.avatar: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		WITH latest_profile_avatars AS (
			SELECT DISTINCT ON (p.account_id)
				p.account_id,
				p.avatar
			FROM users.profiles AS p
			WHERE p.avatar IS NOT NULL
				AND p.avatar != ''
			ORDER BY p.account_id, p.updated_at DESC, p.id DESC
		)
		UPDATE auth.accounts AS a
		SET avatar = latest_profile_avatars.avatar
		FROM latest_profile_avatars
		WHERE a.id = latest_profile_avatars.account_id
			AND (a.avatar IS NULL OR a.avatar = '');
	`); err != nil {
		return fmt.Errorf("backfill auth.accounts.avatar: %w", err)
	}

	return nil
}

func addGlobalAccountAvatarDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.24: Removing global avatar from auth.accounts...")

	_, err := db.ExecContext(ctx, `
		ALTER TABLE auth.accounts
		DROP COLUMN IF EXISTS avatar;
	`)
	if err != nil {
		return fmt.Errorf("drop auth.accounts.avatar: %w", err)
	}

	return nil
}
