package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	CreateTenantTablesVersion     = "1.8.0"
	CreateTenantTablesDescription = "Create tenant schema with traeger and buero tables"
)

func init() {
	MigrationRegistry[CreateTenantTablesVersion] = &Migration{
		Version:     CreateTenantTablesVersion,
		Description: CreateTenantTablesDescription,
		DependsOn:   []string{"1.7.9"}, // Depends on ogs_id being required
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createTenantTables(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropTenantTables(ctx, db)
		},
	)
}

func createTenantTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.0: Creating tenant schema and tables...")

	// Create schema
	_, err := db.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS tenant`)
	if err != nil {
		return fmt.Errorf("error creating tenant schema: %w", err)
	}
	fmt.Println("  Created tenant schema")

	// Create traeger table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS tenant.traeger (
			id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
			name TEXT NOT NULL,
			contact_email TEXT,
			billing_info JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating traeger table: %w", err)
	}
	fmt.Println("  Created tenant.traeger table")

	// Create buero table with FK to traeger
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS tenant.buero (
			id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
			traeger_id TEXT NOT NULL REFERENCES tenant.traeger(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			contact_email TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating buero table: %w", err)
	}
	fmt.Println("  Created tenant.buero table")

	// Add index on foreign key
	_, err = db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_buero_traeger_id ON tenant.buero(traeger_id)
	`)
	if err != nil {
		return fmt.Errorf("error creating buero traeger_id index: %w", err)
	}
	fmt.Println("  Created index idx_buero_traeger_id")

	fmt.Println("Migration 1.8.0: Successfully created tenant tables")
	return nil
}

func dropTenantTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.10: Dropping tenant tables...")

	// Drop in reverse order (FK constraints)
	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS tenant.buero CASCADE`)
	if err != nil {
		return fmt.Errorf("error dropping buero table: %w", err)
	}
	fmt.Println("  Dropped tenant.buero table")

	_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS tenant.traeger CASCADE`)
	if err != nil {
		return fmt.Errorf("error dropping traeger table: %w", err)
	}
	fmt.Println("  Dropped tenant.traeger table")

	_, err = db.ExecContext(ctx, `DROP SCHEMA IF EXISTS tenant CASCADE`)
	if err != nil {
		return fmt.Errorf("error dropping tenant schema: %w", err)
	}
	fmt.Println("  Dropped tenant schema")

	fmt.Println("Migration 1.8.0: Successfully dropped tenant tables")
	return nil
}
