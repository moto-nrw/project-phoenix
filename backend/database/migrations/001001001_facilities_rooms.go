package migrations

import (
	"log"
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	FacilitiesRoomsVersion     = "1.1.1"
	FacilitiesRoomsDescription = "Create facilities.rooms table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[FacilitiesRoomsVersion] = &Migration{
		Version:     FacilitiesRoomsVersion,
		Description: FacilitiesRoomsDescription,
		DependsOn:   []string{"1.0.9"}, // Depends on auth tables
	}

	// Migration 1.1.1: Create facilities.rooms table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createFacilitiesRoomsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropFacilitiesRoomsTable(ctx, db)
		},
	)
}

// createFacilitiesRoomsTable creates the facilities.rooms table
func createFacilitiesRoomsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.1.1: Creating facilities.rooms table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create the facilities.rooms table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS facilities.rooms (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			building TEXT,
			floor INT NOT NULL DEFAULT 0,
			capacity INT NOT NULL DEFAULT 0,
			category TEXT NOT NULL DEFAULT 'Other',
			color TEXT NOT NULL DEFAULT '#FFFFFF',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating facilities.rooms table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for facilities.rooms
		CREATE TRIGGER update_facilities_rooms_updated_at
		BEFORE UPDATE ON facilities.rooms
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating trigger for facilities.rooms table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropFacilitiesRoomsTable drops the facilities.rooms table
func dropFacilitiesRoomsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.1.1: Removing facilities.rooms table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop the trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_facilities_rooms_updated_at ON facilities.rooms;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for facilities.rooms table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS facilities.rooms;
	`)
	if err != nil {
		return fmt.Errorf("error dropping facilities.rooms table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
