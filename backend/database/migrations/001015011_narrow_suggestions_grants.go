package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	narrowSuggestionsGrantsVersion     = "1.15.11"
	narrowSuggestionsGrantsDescription = "Narrow suggestions schema grants for phoenix_auth to operator-specific tables only"
)

func init() {
	MigrationRegistry[narrowSuggestionsGrantsVersion] = &Migration{
		Version:     narrowSuggestionsGrantsVersion,
		Description: narrowSuggestionsGrantsDescription,
		DependsOn:   []string{"1.15.10"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.11: Narrowing suggestions schema grants for phoenix_auth...")

			_, err := db.ExecContext(ctx, `
				-- Revoke write access from user-facing tables that phoenix_auth should not modify
				REVOKE INSERT, UPDATE, DELETE ON suggestions.posts FROM phoenix_auth;
				REVOKE INSERT, UPDATE, DELETE ON suggestions.votes FROM phoenix_auth;
				REVOKE INSERT, UPDATE, DELETE ON suggestions.comments FROM phoenix_auth;

				-- Keep: SELECT on posts, comments (needed for operator to read context)
				-- Keep: Full CRUD on operator_comments, comment_reads, post_reads

				-- Remove overly broad default privileges for future tables
				ALTER DEFAULT PRIVILEGES IN SCHEMA suggestions
					REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM phoenix_auth;
				ALTER DEFAULT PRIVILEGES IN SCHEMA suggestions
					REVOKE USAGE ON SEQUENCES FROM phoenix_auth;
			`)
			if err != nil {
				return fmt.Errorf("error narrowing suggestions grants for phoenix_auth: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.11: Restoring broad suggestions grants for phoenix_auth...")

			_, err := db.ExecContext(ctx, `
				-- Restore write access on user-facing tables
				GRANT INSERT, UPDATE, DELETE ON suggestions.posts TO phoenix_auth;
				GRANT INSERT, UPDATE, DELETE ON suggestions.votes TO phoenix_auth;
				GRANT INSERT, UPDATE, DELETE ON suggestions.comments TO phoenix_auth;

				-- Restore broad default privileges
				ALTER DEFAULT PRIVILEGES IN SCHEMA suggestions
					GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO phoenix_auth;
				ALTER DEFAULT PRIVILEGES IN SCHEMA suggestions
					GRANT USAGE ON SEQUENCES TO phoenix_auth;
			`)
			if err != nil {
				return fmt.Errorf("error restoring broad suggestions grants for phoenix_auth: %w", err)
			}

			return nil
		},
	)
}
