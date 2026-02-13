package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	populateAccountTenantsVersion     = "1.14.6"
	populateAccountTenantsDescription = "Populate account_tenants mapping for all existing accounts to default tenant"
)

func init() {
	MigrationRegistry[populateAccountTenantsVersion] = &Migration{
		Version:     populateAccountTenantsVersion,
		Description: populateAccountTenantsDescription,
		DependsOn:   []string{"1.14.2", "1.13.2"}, // tenant_id on all tables + account_tenants table
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return populateAccountTenants(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackPopulateAccountTenants(ctx, db)
		},
	)
}

func populateAccountTenants(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.14.6: Populating account_tenants for default tenant...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Ensure default organization exists
	_, err = tx.ExecContext(ctx, `
		INSERT INTO platform.organizations (id, name, slug, active)
		VALUES (1, 'Default Organization', 'default', true)
		ON CONFLICT (id) DO NOTHING;
	`)
	if err != nil {
		return fmt.Errorf("error ensuring default organization: %w", err)
	}

	// Ensure default school exists
	_, err = tx.ExecContext(ctx, `
		INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active)
		VALUES (1, 1, 'Default School', 'default', 'default', true)
		ON CONFLICT (id) DO NOTHING;
	`)
	if err != nil {
		return fmt.Errorf("error ensuring default school: %w", err)
	}

	// Map all existing accounts to Tenant 1 (skip already mapped ones)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at)
		SELECT id, 1, 'active', NOW()
		FROM auth.accounts
		WHERE id NOT IN (
			SELECT account_id FROM auth.account_tenants WHERE tenant_id = 1
		);
	`)
	if err != nil {
		return fmt.Errorf("error populating account_tenants: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("Migration 1.14.6: Mapped %d accounts to default tenant (school_id=1)\n", rowsAffected)

	fmt.Println("Migration 1.14.6: Successfully populated account_tenants")
	return tx.Commit()
}

func rollbackPopulateAccountTenants(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.14.6: Removing default tenant mappings...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Remove account_tenants entries for tenant 1
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.account_tenants WHERE tenant_id = 1;
	`)
	if err != nil {
		return fmt.Errorf("error removing account_tenants entries: %w", err)
	}

	// Remove default school and organization (only if they were created by this migration)
	_, err = tx.ExecContext(ctx, `
		DELETE FROM platform.schools WHERE id = 1 AND slug = 'default';
		DELETE FROM platform.organizations WHERE id = 1 AND slug = 'default';
	`)
	if err != nil {
		return fmt.Errorf("error removing default organization/school: %w", err)
	}

	fmt.Println("Migration 1.14.6: Successfully removed default tenant mappings")
	return tx.Commit()
}
