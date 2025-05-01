package migrations

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// registerMigration registers a migration with the global Migrations registry
// and adds it to the MigrationRegistry for metadata tracking
func registerMigration(migration *Migration) {
	// Store in metadata registry
	MigrationRegistry[migration.Version] = migration

	// Register the migration functions with the Bun migrator
	Migrations.MustRegister(
		// Up function
		func(ctx context.Context, db *bun.DB) error {
			startTime := time.Now()
			fmt.Printf("Running migration V%s: %s\n", migration.Version, migration.Description)

			// Run the migration
			err := migration.Up(ctx, db)

			// Record execution in migration_metadata if the table exists
			duration := time.Since(startTime).Milliseconds()
			recordMigration(ctx, db, migration, duration, err)

			if err != nil {
				fmt.Printf("Migration V%s failed: %v\n", migration.Version, err)
				return err
			}

			fmt.Printf("Migration V%s completed successfully (%d ms)\n", migration.Version, duration)
			return nil
		},
		// Down function
		func(ctx context.Context, db *bun.DB) error {
			startTime := time.Now()
			fmt.Printf("Rolling back migration V%s: %s\n", migration.Version, migration.Description)

			// Run the rollback
			err := migration.Down(ctx, db)

			if err != nil {
				fmt.Printf("Rollback of V%s failed: %v\n", migration.Version, err)
				return err
			}

			duration := time.Since(startTime).Milliseconds()
			fmt.Printf("Rollback of V%s completed successfully (%d ms)\n", migration.Version, duration)
			return nil
		},
	)
}

// recordMigration attempts to record migration metadata if the metadata table exists
// This is a best-effort operation that doesn't fail if the table doesn't exist yet
func recordMigration(ctx context.Context, db *bun.DB, migration *Migration, durationMs int64, err error) {
	// Check if the migration_metadata table exists
	var exists bool
	checkErr := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'migration_metadata'
		)
	`).Scan(&exists)

	if checkErr != nil || !exists {
		// Table doesn't exist yet, this is expected for the first migration
		return
	}

	// Record migration metadata
	_, insertErr := db.ExecContext(ctx, `
		INSERT INTO migration_metadata (
			version, description, applied_at, execution_time_ms, 
			checksum, success, error_message, applied_by
		) VALUES (?, ?, now(), ?, ?, ?, ?, current_user)
		ON CONFLICT (version) DO UPDATE SET
			applied_at = now(),
			execution_time_ms = ?,
			success = ?,
			error_message = ?
	`,
		migration.Version,
		migration.Description,
		durationMs,
		"", // checksum - to be implemented
		err == nil,
		errorToString(err),
		durationMs,
		err == nil,
		errorToString(err),
	)

	if insertErr != nil {
		// Log but don't fail the migration
		fmt.Printf("Warning: Failed to record migration metadata: %v\n", insertErr)
	}
}

// errorToString safely converts an error to string
func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Migrate runs all pending migrations
func Migrate() {
	db, err := database.DBConn()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	migrator := migrate.NewMigrator(db, Migrations)

	// Initialize migration tables
	if err := migrator.Init(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Validate migrations before running
	ctx := context.Background()
	if err := ValidateMigrations(ctx, db); err != nil {
		log.Fatalf("Migration validation failed: %v", err)
	}

	// Print migration plan
	PrintMigrationPlan()

	// Run migrations
	group, err := migrator.Migrate(ctx)
	if err != nil {
		log.Fatal(err)
	}

	if group.ID == 0 {
		fmt.Println("No new migrations to run")
	} else {
		fmt.Printf("Migrated to %s\n", group)
	}
}

// MigrateStatus shows current migration status
func MigrateStatus() {
	db, err := database.DBConn()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	migrator := migrate.NewMigrator(db, Migrations)

	// Initialize migration tables
	if err := migrator.Init(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Get status
	ms, err := migrator.MigrationsWithStatus(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Migration Status:")
	fmt.Println("=================")

	if len(ms) == 0 {
		fmt.Println("No migrations found")
		return
	}

	for _, m := range ms {
		status := "PENDING"
		// Check if migration is applied
		if m.MigratedAt.Unix() > 0 {
			status = "APPLIED"
		}

		// Get metadata from our registry if available
		meta, exists := MigrationRegistry[m.Name]
		desc := ""
		if exists {
			desc = fmt.Sprintf(" - %s", meta.Description)
		}

		fmt.Printf("V%s: %s%s\n", m.Name, status, desc)
	}
}

// Reset drops all tables and re-runs all migrations
// CAUTION: This will delete all data
func Reset() {
	// First reset the database by dropping all tables
	err := ResetDatabase()
	if err != nil {
		log.Fatalf("Failed to reset database: %v", err)
	}

	// Then run all migrations
	db, err := database.DBConn()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Initialize new migrator
	migrator := migrate.NewMigrator(db, Migrations)

	if err := migrator.Init(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Run migrations
	fmt.Println("Running all migrations...")
	group, err := migrator.Migrate(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Database reset and migration completed successfully. Migrated to %s\n", group)
}
