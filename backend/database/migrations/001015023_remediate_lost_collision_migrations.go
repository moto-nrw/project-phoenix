package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	remediateLostCollisionMigrationsVersion     = "1.15.23"
	remediateLostCollisionMigrationsDescription = "Remediate schema changes lost to migration version collisions (persons soft-delete, announcement targeting)"
)

func init() {
	MigrationRegistry[remediateLostCollisionMigrationsVersion] = &Migration{
		Version:     remediateLostCollisionMigrationsVersion,
		Description: remediateLostCollisionMigrationsDescription,
		DependsOn:   []string{"1.15.22"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return remediateLostCollisionMigrationsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return remediateLostCollisionMigrationsDown(ctx, db)
		},
	)
}

func remediateLostCollisionMigrationsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.23: Remediating schema changes lost to version collisions...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// 1. Add deleted_at column to users.persons for soft-delete support
	// (from lost migration 1.15.20 — persons soft-delete)
	fmt.Println("Migration 1.15.23: Adding deleted_at column to users.persons (IF NOT EXISTS)...")
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.persons ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
	`)
	if err != nil {
		return fmt.Errorf("error adding deleted_at to users.persons: %w", err)
	}

	// 2. Replace unique indexes on persons to be partial (exclude soft-deleted rows)
	fmt.Println("Migration 1.15.23: Replacing persons unique indexes with partial indexes...")
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_persons_tenant_tag;
		DROP INDEX IF EXISTS users.idx_persons_tenant_account;

		CREATE UNIQUE INDEX IF NOT EXISTS idx_persons_tenant_tag
		ON users.persons (tenant_id, tag_id) WHERE deleted_at IS NULL AND tag_id IS NOT NULL;

		CREATE UNIQUE INDEX IF NOT EXISTS idx_persons_tenant_account
		ON users.persons (tenant_id, account_id) WHERE deleted_at IS NULL AND account_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error replacing persons unique indexes: %w", err)
	}

	// 3. Add announcement targeting columns
	// (from lost migration 1.15.22 — announcement org/tenant targeting)
	// Using IF NOT EXISTS since migration 1.15.22 also adds these columns,
	// and we don't know which runs first on a given environment.
	fmt.Println("Migration 1.15.23: Adding announcement targeting columns (IF NOT EXISTS)...")
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE platform.announcements ADD COLUMN IF NOT EXISTS target_org_ids BIGINT[] NOT NULL DEFAULT '{}';
		ALTER TABLE platform.announcements ADD COLUMN IF NOT EXISTS target_tenant_ids BIGINT[] NOT NULL DEFAULT '{}';

		CREATE INDEX IF NOT EXISTS idx_announcements_target_org_ids ON platform.announcements USING GIN(target_org_ids);
		CREATE INDEX IF NOT EXISTS idx_announcements_target_tenant_ids ON platform.announcements USING GIN(target_tenant_ids);

		COMMENT ON COLUMN platform.announcements.target_org_ids IS 'Array of organization IDs that can see this announcement. Empty array means no org filter (all orgs).';
		COMMENT ON COLUMN platform.announcements.target_tenant_ids IS 'Array of tenant (school) IDs that can see this announcement. Empty array means no tenant filter (all tenants).';
	`)
	if err != nil {
		return fmt.Errorf("error adding announcement targeting columns: %w", err)
	}

	fmt.Println("Migration 1.15.23: Remediation complete")
	return tx.Commit()
}

func remediateLostCollisionMigrationsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.23: Reverting remediated schema changes...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// 1. Drop partial indexes and recreate original unique indexes on persons
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_persons_tenant_tag;
		DROP INDEX IF EXISTS users.idx_persons_tenant_account;

		CREATE UNIQUE INDEX idx_persons_tenant_tag
		ON users.persons(tenant_id, tag_id);

		CREATE UNIQUE INDEX idx_persons_tenant_account
		ON users.persons(tenant_id, account_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring persons unique indexes: %w", err)
	}

	// 2. Drop deleted_at column from users.persons
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.persons DROP COLUMN IF EXISTS deleted_at;
	`)
	if err != nil {
		return fmt.Errorf("error dropping deleted_at from users.persons: %w", err)
	}

	// 3. Drop announcement targeting indexes and columns
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS platform.idx_announcements_target_org_ids;
		DROP INDEX IF EXISTS platform.idx_announcements_target_tenant_ids;

		ALTER TABLE platform.announcements DROP COLUMN IF EXISTS target_org_ids;
		ALTER TABLE platform.announcements DROP COLUMN IF EXISTS target_tenant_ids;
	`)
	if err != nil {
		return fmt.Errorf("error dropping announcement targeting columns: %w", err)
	}

	fmt.Println("Migration 1.15.23: Rollback complete")
	return tx.Commit()
}
