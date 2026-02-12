package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	createOrgsAndSchoolsVersion     = "1.13.1"
	createOrgsAndSchoolsDescription = "Create platform.organizations and platform.schools tables for multi-tenancy"
)

func init() {
	MigrationRegistry[createOrgsAndSchoolsVersion] = &Migration{
		Version:     createOrgsAndSchoolsVersion,
		Description: createOrgsAndSchoolsDescription,
		DependsOn:   []string{"1.11.1"}, // Depends on platform schema
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createOrganizationsAndSchools(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackOrganizationsAndSchools(ctx, db)
		},
	)
}

func createOrganizationsAndSchools(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.13.1: Creating platform.organizations and platform.schools tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create organizations table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS platform.organizations (
			id              BIGSERIAL PRIMARY KEY,
			name            VARCHAR(200) NOT NULL,
			slug            VARCHAR(100) NOT NULL UNIQUE,
			active          BOOLEAN NOT NULL DEFAULT true,
			settings        JSONB DEFAULT '{}',
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating platform.organizations table: %w", err)
	}

	// Create schools table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS platform.schools (
			id                BIGSERIAL PRIMARY KEY,
			organization_id   BIGINT NOT NULL REFERENCES platform.organizations(id),
			name              VARCHAR(200) NOT NULL,
			slug              VARCHAR(100) NOT NULL,
			subdomain         VARCHAR(100) NOT NULL UNIQUE,
			active            BOOLEAN NOT NULL DEFAULT true,
			settings          JSONB DEFAULT '{}',
			address           TEXT,
			city              VARCHAR(100),
			zip               VARCHAR(20),
			phone             VARCHAR(50),
			email             VARCHAR(255),
			device_pin_hash   VARCHAR(255),
			created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(organization_id, slug)
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating platform.schools table: %w", err)
	}

	// Create indexes
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_schools_subdomain ON platform.schools(subdomain);
		CREATE INDEX IF NOT EXISTS idx_schools_organization ON platform.schools(organization_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for platform.schools: %w", err)
	}

	// Create updated_at triggers
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_platform_organizations_updated_at ON platform.organizations;
		CREATE TRIGGER update_platform_organizations_updated_at
		BEFORE UPDATE ON platform.organizations
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_platform_schools_updated_at ON platform.schools;
		CREATE TRIGGER update_platform_schools_updated_at
		BEFORE UPDATE ON platform.schools
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	fmt.Println("Migration 1.13.1: Successfully created platform.organizations and platform.schools tables")
	return tx.Commit()
}

func rollbackOrganizationsAndSchools(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.13.1: Dropping organizations and schools tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop triggers first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_platform_schools_updated_at ON platform.schools;
		DROP TRIGGER IF EXISTS update_platform_organizations_updated_at ON platform.organizations;
	`)
	if err != nil {
		return fmt.Errorf("error dropping triggers: %w", err)
	}

	// Drop tables (schools first due to FK)
	_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS platform.schools CASCADE;`)
	if err != nil {
		return fmt.Errorf("error dropping platform.schools table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS platform.organizations CASCADE;`)
	if err != nil {
		return fmt.Errorf("error dropping platform.organizations table: %w", err)
	}

	fmt.Println("Migration 1.13.1: Successfully rolled back organizations and schools tables")
	return tx.Commit()
}
