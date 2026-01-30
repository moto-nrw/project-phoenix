package migrations

import (
	"context"
	"fmt"
	"log"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/uptrace/bun/migrate"
)

// Migrate runs all pending migrations
func Migrate() {
	db, err := database.DBConn()
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	migrator := migrate.NewMigrator(db, Migrations)

	// Initialize migration tables
	if err := migrator.Init(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Validate migrations before running
	if err := ValidateMigrations(); err != nil {
		log.Fatalf("Migration validation failed: %v", err)
	}
	ctx := context.Background()

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
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

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
