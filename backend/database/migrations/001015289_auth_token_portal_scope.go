package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	authTokenPortalScopeVersion     = "1.15.289"
	authTokenPortalScopeDescription = "Persist the portal scope that minted each refresh-token session"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     authTokenPortalScopeVersion,
		Description: authTokenPortalScopeDescription,
		DependsOn:   []string{"1.15.288"},
	})
	Migrations.MustRegister(authTokenPortalScopeUp, authTokenPortalScopeDown)
}

func authTokenPortalScopeUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.289: Adding portal scope to refresh-token sessions...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE auth.tokens
			ADD COLUMN IF NOT EXISTS portal_scope VARCHAR(16) NOT NULL DEFAULT 'unknown';
		ALTER TABLE auth.tokens
			DROP CONSTRAINT IF EXISTS chk_tokens_portal_scope;
		ALTER TABLE auth.tokens
			ADD CONSTRAINT chk_tokens_portal_scope
			CHECK (portal_scope IN ('tenant', 'org', 'parent', 'school', 'unknown'));
		COMMENT ON COLUMN auth.tokens.portal_scope IS
			'Isolated portal that minted the session; unknown identifies pre-1.15.289 tokens';
	`)
	if err != nil {
		return fmt.Errorf("add auth token portal scope: %w", err)
	}
	return nil
}

func authTokenPortalScopeDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `ALTER TABLE auth.tokens DROP COLUMN IF EXISTS portal_scope;`)
	if err != nil {
		return fmt.Errorf("drop auth token portal scope: %w", err)
	}
	return nil
}
