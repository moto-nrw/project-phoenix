package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	rolesBaseRoleVersion     = "1.15.30"
	rolesBaseRoleDescription = "Add base_role column to auth.roles for mapping custom roles to system roles (announcement targeting)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     rolesBaseRoleVersion,
		Description: rolesBaseRoleDescription,
		DependsOn:   []string{"1.15.29", "1.0.4"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addRolesBaseRole(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackRolesBaseRole(ctx, db)
		},
	)
}

func addRolesBaseRole(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.30: Adding base_role column to auth.roles...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.roles
			ADD COLUMN IF NOT EXISTS base_role VARCHAR;
	`)
	if err != nil {
		return fmt.Errorf("failed to add base_role column: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DO $$ BEGIN
			ALTER TABLE auth.roles
				ADD CONSTRAINT chk_roles_base_role
				CHECK (base_role IS NULL OR base_role IN ('admin', 'user', 'guardian'));
		EXCEPTION WHEN duplicate_object THEN NULL;
		END $$;
	`)
	if err != nil {
		return fmt.Errorf("failed to add check constraint: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_roles_base_role
			ON auth.roles(base_role)
			WHERE base_role IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	fmt.Println("  Added base_role column with check constraint and partial index")
	return tx.Commit()
}

func rollbackRolesBaseRole(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rollback 1.15.30: Removing base_role column from auth.roles...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.roles
			DROP COLUMN IF EXISTS base_role;
	`)
	if err != nil {
		return fmt.Errorf("failed to drop base_role column: %w", err)
	}

	fmt.Println("  Removed base_role column successfully")
	return tx.Commit()
}
