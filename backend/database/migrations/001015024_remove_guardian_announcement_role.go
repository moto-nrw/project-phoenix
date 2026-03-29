package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	removeGuardianAnnouncementRoleVersion     = "1.15.24"
	removeGuardianAnnouncementRoleDescription = "Remove guardian from announcement target_roles (no guardian frontend exists)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     removeGuardianAnnouncementRoleVersion,
		Description: removeGuardianAnnouncementRoleDescription,
		DependsOn:   []string{"1.15.23"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createRemoveGuardianAnnouncementRole(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackRemoveGuardianAnnouncementRole(ctx, db)
		},
	)
}

func createRemoveGuardianAnnouncementRole(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.24: Removing guardian from announcement target_roles...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Remove 'guardian' from any existing target_roles arrays
	_, err = tx.ExecContext(ctx, `
		UPDATE platform.announcements
		SET target_roles = array_remove(target_roles, 'guardian')
		WHERE 'guardian' = ANY(target_roles);
	`)
	if err != nil {
		return fmt.Errorf("error removing guardian from target_roles: %w", err)
	}

	// Update the column comment to reflect valid roles
	_, err = tx.ExecContext(ctx, `
		COMMENT ON COLUMN platform.announcements.target_roles IS
		'Array of system role names (admin, user) that can see this announcement. Empty array means all roles.';
	`)
	if err != nil {
		return fmt.Errorf("error updating column comment: %w", err)
	}

	fmt.Println("Migration 1.15.24: Successfully removed guardian from announcement target_roles")
	return tx.Commit()
}

func rollbackRemoveGuardianAnnouncementRole(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.24: Restoring guardian column comment...")

	// Restore original comment — we cannot restore removed array values
	_, err := db.ExecContext(ctx, `
		COMMENT ON COLUMN platform.announcements.target_roles IS
		'Array of system role names (admin, user, guardian) that can see this announcement. Empty array means all roles.';
	`)
	if err != nil {
		return fmt.Errorf("error restoring column comment: %w", err)
	}

	return nil
}
