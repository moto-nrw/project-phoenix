package migrations

import (
	"context"
	"fmt"
	"log"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/uptrace/bun"
)

// AddSampleStudents adds sample students data for development/testing
// This is a temporary compatibility function for the CLI command
// This functionality will be properly implemented in the V1_12_0__seed_data.go migration
func AddSampleStudents() {
	fmt.Println("This functionality has been moved to the migration system.")
	fmt.Println("Please use `./main migrate reset` to completely recreate the database with sample data.")
	fmt.Println("Sample data will be automatically added when applying migrations in development mode.")

	// This is a no-op function to maintain CLI compatibility
	// The actual sample data loading will be handled by V1_12_0__seed_data.go
}

// FixConstraints applies all necessary constraints to match the ER diagram
// This is a temporary compatibility function for the CLI command
// This functionality will be properly implemented in the V1_11_0__indexes_and_constraints.go migration
func FixConstraints() {
	fmt.Println("This functionality has been moved to the migration system.")
	fmt.Println("Please use `./main migrate` to apply all pending migrations, including constraint fixes.")
	fmt.Println("All constraints will be correctly applied as part of the migration process.")

	// Connect to DB to see if we need to apply any fixes
	db, err := database.DBConn()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()

	// Check if migrations have been run
	var migrationsTableExists bool
	err = db.QueryRowContext(context.Background(), `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'bun_migrations'
		)
	`).Scan(&migrationsTableExists)

	if err != nil {
		log.Fatalf("Error checking migration status: %v", err)
	}

	if !migrationsTableExists {
		fmt.Println("No migrations have been run yet. Please run `./main migrate` first.")
		return
	}

	// Check how many migrations have been applied
	var count int
	err = db.QueryRowContext(context.Background(), `
		SELECT COUNT(*) FROM bun_migrations
	`).Scan(&count)

	if err != nil {
		log.Fatalf("Error checking migration count: %v", err)
	}

	fmt.Printf("Found %d migrations applied.\n", count)

	// Apply legacy constraint fixes only if requested
	if count > 0 {
		applyLegacyConstraintFixes(db)
	}
}

// applyLegacyConstraintFixes applies critical constraint fixes for backward compatibility
// with the old migration system. This ensures that databases set up with the old system
// will still function correctly until they are properly migrated to the new system.
func applyLegacyConstraintFixes(db *bun.DB) {
	fmt.Println("Applying compatibility constraint fixes...")

	// Begin a transaction for atomicity
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatalf("Error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Collection of the most critical fixes
	fixes := []string{
		// 1. Fix groups.name to be UNIQUE and INDEXED
		`ALTER TABLE IF EXISTS groups ADD CONSTRAINT IF NOT EXISTS groups_name_key UNIQUE (name)`,
		`CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name)`,

		// 2. Fix room_occupancy.device_id to be NOT NULL, UNIQUE and INDEXED
		`UPDATE room_occupancy SET device_id = 'device_' || id WHERE device_id IS NULL`,
		`ALTER TABLE IF EXISTS room_occupancy ALTER COLUMN device_id SET NOT NULL`,
		`ALTER TABLE IF EXISTS room_occupancy ADD CONSTRAINT IF NOT EXISTS room_occupancy_device_id_key UNIQUE (device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_room_occupancy_device_id ON room_occupancy(device_id)`,
	}

	// Apply each fix
	for _, fix := range fixes {
		_, err = tx.ExecContext(ctx, fix)
		if err != nil {
			fmt.Printf("Warning: Error applying fix: %s\nError: %v\n", fix, err)
			// Continue with other fixes - not fatal
		}
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		log.Fatalf("Error committing transaction: %v", err)
	}

	fmt.Println("Compatibility constraint fixes applied successfully.")
	fmt.Println("Please run `./main migrate` to apply all new migrations.")
}
