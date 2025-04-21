package migrations

import (
	"context"
	"fmt"
	"log"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/uptrace/bun/migrate"
)

// Migrate runs all migrations
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

	// Run migrations
	group, err := migrator.Migrate(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	if group.ID == 0 {
		fmt.Println("No new migrations to run")
	} else {
		fmt.Printf("Migrated to %s\n", group)
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
