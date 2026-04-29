package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createEnrollmentFormSchemasVersion     = "1.15.48"
	createEnrollmentFormSchemasDescription = "Create enrollment schema + enrollment.form_schemas table for versioned parent-enrollment form definitions (PR 5)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createEnrollmentFormSchemasVersion,
		Description: createEnrollmentFormSchemasDescription,
		DependsOn: []string{
			"1.15.1", // RLS infrastructure
			"1.0.1",  // auth.accounts (created_by FK)
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createEnrollmentFormSchemasUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return createEnrollmentFormSchemasDown(ctx, db)
		},
	)
}

func createEnrollmentFormSchemasUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.48: Creating enrollment schema + enrollment.form_schemas table...")

	if _, err := db.NewRaw(`CREATE SCHEMA IF NOT EXISTS enrollment;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating enrollment schema: %w", err)
	}

	// Grant USAGE on the new schema so phoenix_tenant + phoenix_admin can
	// reach tables inside. Future tables in the schema get default
	// privileges via a follow-up ALTER DEFAULT PRIVILEGES below — same
	// pattern as 1.14.1's bulk schema setup.
	if _, err := db.NewRaw(`
		GRANT USAGE ON SCHEMA enrollment TO phoenix_tenant, phoenix_admin;
		ALTER DEFAULT PRIVILEGES IN SCHEMA enrollment
			GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO phoenix_tenant;
		ALTER DEFAULT PRIVILEGES IN SCHEMA enrollment
			GRANT USAGE ON SEQUENCES TO phoenix_tenant;
		ALTER DEFAULT PRIVILEGES IN SCHEMA enrollment
			GRANT ALL ON TABLES TO phoenix_admin;
		ALTER DEFAULT PRIVILEGES IN SCHEMA enrollment
			GRANT ALL ON SEQUENCES TO phoenix_admin;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed granting permissions on enrollment schema: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS enrollment.form_schemas (
			id          BIGSERIAL PRIMARY KEY,
			tenant_id   BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			version     INT NOT NULL,
			fields      JSONB NOT NULL DEFAULT '[]'::jsonb,
			is_active   BOOLEAN NOT NULL DEFAULT FALSE,
			created_by  BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE RESTRICT,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT unique_form_schema_version UNIQUE (tenant_id, version)
		);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating form_schemas: %w", err)
	}

	// At most one active schema per tenant. Partial unique index enforces.
	if _, err := db.NewRaw(`
		CREATE UNIQUE INDEX IF NOT EXISTS uq_form_schemas_one_active_per_tenant
		ON enrollment.form_schemas (tenant_id)
		WHERE is_active = TRUE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating active-schema unique index: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE enrollment.form_schemas ENABLE ROW LEVEL SECURITY;
		ALTER TABLE enrollment.form_schemas FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_enrollment_form_schemas ON enrollment.form_schemas
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed enabling RLS on form_schemas: %w", err)
	}

	return nil
}

func createEnrollmentFormSchemasDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.48: Dropping enrollment.form_schemas...")
	if _, err := db.NewRaw(`DROP TABLE IF EXISTS enrollment.form_schemas CASCADE;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping form_schemas: %w", err)
	}
	// Don't drop the enrollment schema — later migrations may share it.
	return nil
}
