package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	announcementTargetRolesVersion     = "1.12.4"
	announcementTargetRolesDescription = "Add target_roles column to announcements for role-based targeting"
)

func init() {
	MigrationRegistry[announcementTargetRolesVersion] = &Migration{
		Version:     announcementTargetRolesVersion,
		Description: announcementTargetRolesDescription,
		DependsOn:   []string{"1.12.3"}, // Depends on post_reads
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAnnouncementTargetRoles(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackAnnouncementTargetRoles(ctx, db)
		},
	)
}

func createAnnouncementTargetRoles(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.12.4: Adding target_roles column to announcements...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop unused target_school_ids column if it exists
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE platform.announcements
		DROP COLUMN IF EXISTS target_school_ids;
	`)
	if err != nil {
		return fmt.Errorf("error dropping target_school_ids column: %w", err)
	}

	// Add target_roles column (empty array = all roles can see)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE platform.announcements
		ADD COLUMN IF NOT EXISTS target_roles TEXT[] NOT NULL DEFAULT '{}';
	`)
	if err != nil {
		return fmt.Errorf("error adding target_roles column: %w", err)
	}

	// Create GIN index for efficient array queries
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_announcements_target_roles
		ON platform.announcements USING GIN(target_roles);
	`)
	if err != nil {
		return fmt.Errorf("error creating GIN index for target_roles: %w", err)
	}

	// Add comment
	_, err = tx.ExecContext(ctx, `
		COMMENT ON COLUMN platform.announcements.target_roles IS
		'Array of system role names (admin, user, guardian) that can see this announcement. Empty array means all roles.';
	`)
	if err != nil {
		return fmt.Errorf("error adding column comment: %w", err)
	}

	fmt.Println("Migration 1.12.4: Successfully added target_roles column")
	return tx.Commit()
}

func rollbackAnnouncementTargetRoles(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.12.4: Removing target_roles column...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop index
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS platform.idx_announcements_target_roles;
	`)
	if err != nil {
		return fmt.Errorf("error dropping target_roles index: %w", err)
	}

	// Drop column
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE platform.announcements
		DROP COLUMN IF EXISTS target_roles;
	`)
	if err != nil {
		return fmt.Errorf("error dropping target_roles column: %w", err)
	}

	// Re-add target_school_ids for backwards compatibility
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE platform.announcements
		ADD COLUMN IF NOT EXISTS target_school_ids BIGINT[] DEFAULT '{}';
	`)
	if err != nil {
		return fmt.Errorf("error re-adding target_school_ids column: %w", err)
	}

	fmt.Println("Migration 1.12.4: Successfully removed target_roles column")
	return tx.Commit()
}
