package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	announcementOrgTenantTargetingVersion     = "1.15.20"
	announcementOrgTenantTargetingDescription = "Add target_org_ids and target_tenant_ids columns to announcements for org/tenant-scoped targeting"
)

func init() {
	MigrationRegistry[announcementOrgTenantTargetingVersion] = &Migration{
		Version:     announcementOrgTenantTargetingVersion,
		Description: announcementOrgTenantTargetingDescription,
		DependsOn:   []string{"1.15.19"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAnnouncementOrgTenantTargeting(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackAnnouncementOrgTenantTargeting(ctx, db)
		},
	)
}

func createAnnouncementOrgTenantTargeting(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.20: Adding target_org_ids and target_tenant_ids columns to announcements...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Add target_org_ids column (empty array = no org filter, visible to all orgs)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE platform.announcements
		ADD COLUMN IF NOT EXISTS target_org_ids BIGINT[] NOT NULL DEFAULT '{}';
	`)
	if err != nil {
		return fmt.Errorf("error adding target_org_ids column: %w", err)
	}

	// Add target_tenant_ids column (empty array = no tenant filter, visible to all tenants)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE platform.announcements
		ADD COLUMN IF NOT EXISTS target_tenant_ids BIGINT[] NOT NULL DEFAULT '{}';
	`)
	if err != nil {
		return fmt.Errorf("error adding target_tenant_ids column: %w", err)
	}

	// Create GIN indexes for efficient array queries
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_announcements_target_org_ids
		ON platform.announcements USING GIN(target_org_ids);
	`)
	if err != nil {
		return fmt.Errorf("error creating GIN index for target_org_ids: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_announcements_target_tenant_ids
		ON platform.announcements USING GIN(target_tenant_ids);
	`)
	if err != nil {
		return fmt.Errorf("error creating GIN index for target_tenant_ids: %w", err)
	}

	// Add comments
	_, err = tx.ExecContext(ctx, `
		COMMENT ON COLUMN platform.announcements.target_org_ids IS
		'Array of organization IDs that can see this announcement. Empty array means no org filter (all orgs).';
	`)
	if err != nil {
		return fmt.Errorf("error adding target_org_ids column comment: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		COMMENT ON COLUMN platform.announcements.target_tenant_ids IS
		'Array of tenant (school) IDs that can see this announcement. Empty array means no tenant filter (all tenants).';
	`)
	if err != nil {
		return fmt.Errorf("error adding target_tenant_ids column comment: %w", err)
	}

	fmt.Println("Migration 1.15.20: Successfully added target_org_ids and target_tenant_ids columns")
	return tx.Commit()
}

func rollbackAnnouncementOrgTenantTargeting(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.20: Removing target_org_ids and target_tenant_ids columns...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop indexes
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS platform.idx_announcements_target_org_ids;
	`)
	if err != nil {
		return fmt.Errorf("error dropping target_org_ids index: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS platform.idx_announcements_target_tenant_ids;
	`)
	if err != nil {
		return fmt.Errorf("error dropping target_tenant_ids index: %w", err)
	}

	// Drop columns
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE platform.announcements
		DROP COLUMN IF EXISTS target_org_ids;
	`)
	if err != nil {
		return fmt.Errorf("error dropping target_org_ids column: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE platform.announcements
		DROP COLUMN IF EXISTS target_tenant_ids;
	`)
	if err != nil {
		return fmt.Errorf("error dropping target_tenant_ids column: %w", err)
	}

	fmt.Println("Migration 1.15.20: Successfully removed target_org_ids and target_tenant_ids columns")
	return tx.Commit()
}
