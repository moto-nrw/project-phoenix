package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	narrowSuggestionsGrantsVersion     = "1.15.11"
	narrowSuggestionsGrantsDescription = "Remove overly broad default privileges for suggestions schema"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     narrowSuggestionsGrantsVersion,
		Description: narrowSuggestionsGrantsDescription,
		DependsOn:   []string{"1.15.10"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.11: Removing overly broad default privileges for suggestions schema...")

			// The blanket ALTER DEFAULT PRIVILEGES from 1.15.10 auto-grants
			// full CRUD on any future table in the suggestions schema.
			// Remove this so new tables require explicit grants in their
			// creating migration. Existing table grants are preserved —
			// phoenix_auth still needs write access to all current tables
			// because both user and operator requests run as this role.
			_, err := db.ExecContext(ctx, `
				ALTER DEFAULT PRIVILEGES IN SCHEMA suggestions
					REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM phoenix_auth;
				ALTER DEFAULT PRIVILEGES IN SCHEMA suggestions
					REVOKE USAGE ON SEQUENCES FROM phoenix_auth;
			`)
			if err != nil {
				return fmt.Errorf("error removing default privileges for suggestions schema: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.11: Restoring broad default privileges for suggestions schema...")

			_, err := db.ExecContext(ctx, `
				ALTER DEFAULT PRIVILEGES IN SCHEMA suggestions
					GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO phoenix_auth;
				ALTER DEFAULT PRIVILEGES IN SCHEMA suggestions
					GRANT USAGE ON SEQUENCES TO phoenix_auth;
			`)
			if err != nil {
				return fmt.Errorf("error restoring default privileges for suggestions schema: %w", err)
			}

			return nil
		},
	)
}
