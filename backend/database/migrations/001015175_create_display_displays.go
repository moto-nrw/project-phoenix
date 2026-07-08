package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createDisplayDisplaysVersion     = "1.15.175"
	createDisplayDisplaysDescription = "Create display schema + display.displays table for token-authenticated info-point dashboards (issue #1325)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createDisplayDisplaysVersion,
		Description: createDisplayDisplaysDescription,
		DependsOn: []string{
			"1.15.1", // RLS infrastructure
			"1.0.1",  // platform.schools (tenant_id FK)
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createDisplayDisplaysUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return createDisplayDisplaysDown(ctx, db)
		},
	)
}

func createDisplayDisplaysUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.175: Creating display schema + display.displays table...")

	if _, err := db.NewRaw(`CREATE SCHEMA IF NOT EXISTS display;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating display schema: %w", err)
	}

	// Grant USAGE on the new schema so phoenix_tenant + phoenix_admin can
	// reach tables inside. Future tables in the schema get default
	// privileges via a follow-up ALTER DEFAULT PRIVILEGES below - same
	// pattern as 1.15.59's enrollment schema setup.
	if _, err := db.NewRaw(`
		GRANT USAGE ON SCHEMA display TO phoenix_tenant, phoenix_admin;
		ALTER DEFAULT PRIVILEGES IN SCHEMA display
			GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO phoenix_tenant;
		ALTER DEFAULT PRIVILEGES IN SCHEMA display
			GRANT USAGE ON SEQUENCES TO phoenix_tenant;
		ALTER DEFAULT PRIVILEGES IN SCHEMA display
			GRANT ALL ON TABLES TO phoenix_admin;
		ALTER DEFAULT PRIVILEGES IN SCHEMA display
			GRANT ALL ON SEQUENCES TO phoenix_admin;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed granting permissions on display schema: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS display.displays (
			id         BIGSERIAL PRIMARY KEY,
			tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			name       TEXT NOT NULL,
			token_hash TEXT NOT NULL,
			is_active  BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating display.displays: %w", err)
	}

	// The token hash is globally unique (no tenant_id in the index): the
	// public dashboard endpoint resolves tenant FROM the token, so the
	// lookup must be unambiguous across tenants.
	if _, err := db.NewRaw(`
		CREATE UNIQUE INDEX IF NOT EXISTS uq_display_displays_token_hash
			ON display.displays (token_hash);
		CREATE INDEX IF NOT EXISTS idx_display_displays_tenant
			ON display.displays (tenant_id);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating display.displays indexes: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE display.displays ENABLE ROW LEVEL SECURITY;
		ALTER TABLE display.displays FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_display_displays ON display.displays
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed enabling RLS on display.displays: %w", err)
	}

	return nil
}

func createDisplayDisplaysDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.175: Dropping display.displays...")
	if _, err := db.NewRaw(`DROP TABLE IF EXISTS display.displays CASCADE;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping display.displays: %w", err)
	}
	// Don't drop the display schema - later migrations may share it.
	return nil
}
