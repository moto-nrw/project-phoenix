package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/uptrace/bun"
)

const (
	ActivitiesCategoriesVersion     = "1.3.1"
	ActivitiesCategoriesDescription = "Create activities.categories table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ActivitiesCategoriesVersion] = &Migration{
		Version:     ActivitiesCategoriesVersion,
		Description: ActivitiesCategoriesDescription,
		DependsOn:   []string{"1.0.9"}, // Depends on auth tables
	}

	// Migration 1.3.1: Create activities.categories table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createActivitiesCategoriesTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropActivitiesCategoriesTable(ctx, db)
		},
	)
}

// createActivitiesCategoriesTable creates the activities.categories table
func createActivitiesCategoriesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.3.1: Creating activities.categories table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			if logging.Logger != nil {
				logging.Logger.Warnf("Error rolling back transaction: %v", err)
			}
		}
	}()

	// Create the categories table - for categorizing activities
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS activities.categories (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT,
			color TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating categories table: %w", err)
	}

	// Create indexes for categories
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_categories_name ON activities.categories(name);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for categories table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for categories
		DROP TRIGGER IF EXISTS update_activity_categories_updated_at ON activities.categories;
		CREATE TRIGGER update_activity_categories_updated_at
		BEFORE UPDATE ON activities.categories
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating trigger for categories table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropActivitiesCategoriesTable drops the activities.categories table
func dropActivitiesCategoriesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.3.1: Removing activities.categories table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			if logging.Logger != nil {
				logging.Logger.Warnf("Error rolling back transaction: %v", err)
			}
		}
	}()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_activity_categories_updated_at ON activities.categories;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for categories table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS activities.categories;
	`)
	if err != nil {
		return fmt.Errorf("error dropping activities.categories table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
