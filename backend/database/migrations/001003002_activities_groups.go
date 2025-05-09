package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	ActivitiesGroupsVersion     = "1.3.2"
	ActivitiesGroupsDescription = "Create activities.groups table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ActivitiesGroupsVersion] = &Migration{
		Version:     ActivitiesGroupsVersion,
		Description: ActivitiesGroupsDescription,
		DependsOn:   []string{"1.3.1", "1.1.1"}, // Depends on categories and rooms
	}

	// Migration 1.3.2: Create activities.groups table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createActivitiesGroupsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropActivitiesGroupsTable(ctx, db)
		},
	)
}

// createActivitiesGroupsTable creates the activities.groups table
func createActivitiesGroupsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.3.2: Creating activities.groups table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the groups table - activity groups
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS activities.groups (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			max_participants INT NOT NULL,
			is_open BOOLEAN NOT NULL DEFAULT FALSE,
			category_id BIGINT NOT NULL,
			planned_room_id BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_activity_groups_category FOREIGN KEY (category_id) 
				REFERENCES activities.categories(id) ON DELETE RESTRICT,
			CONSTRAINT fk_activity_groups_planned_room FOREIGN KEY (planned_room_id)
				REFERENCES facilities.rooms(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating groups table: %w", err)
	}

	// Create indexes for groups
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_activity_groups_name ON activities.groups(name);
		CREATE INDEX IF NOT EXISTS idx_activity_groups_supervisor ON activities.groups(supervisor_id);
		CREATE INDEX IF NOT EXISTS idx_activity_groups_category ON activities.groups(category_id);
		CREATE INDEX IF NOT EXISTS idx_activity_groups_open ON activities.groups(is_open);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for groups table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for groups
		DROP TRIGGER IF EXISTS update_activity_groups_updated_at ON activities.groups;
		CREATE TRIGGER update_activity_groups_updated_at
		BEFORE UPDATE ON activities.groups
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating trigger for groups table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropActivitiesGroupsTable drops the activities.groups table
func dropActivitiesGroupsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.3.2: Removing activities.groups table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_activity_groups_updated_at ON activities.groups;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for groups table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS activities.groups;
	`)
	if err != nil {
		return fmt.Errorf("error dropping activities.groups table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
