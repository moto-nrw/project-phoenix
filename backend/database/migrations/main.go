package migrations

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/uptrace/bun/migrate"
)

// Migrate runs all pending migrations
func Migrate() {
	db, err := database.DBConn()
	if err != nil {
		logging.Logger.WithError(err).Fatal("failed to connect to database")
	}
	defer func() { _ = db.Close() }()

	migrator := migrate.NewMigrator(db, Migrations)

	// Initialize migration tables
	if err := migrator.Init(context.Background()); err != nil {
		logging.Logger.WithError(err).Fatal("failed to initialize migrator")
	}

	// Validate migrations before running
	ctx := context.Background()
	if err := ValidateMigrations(ctx, db); err != nil {
		logging.Logger.WithError(err).Fatal("migration validation failed")
	}

	// Print migration plan
	PrintMigrationPlan()

	// Run migrations
	group, err := migrator.Migrate(ctx)
	if err != nil {
		logging.Logger.WithError(err).Fatal("migration failed")
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
		logging.Logger.WithError(err).Fatal("failed to connect to database")
	}
	defer func() { _ = db.Close() }()

	migrator := migrate.NewMigrator(db, Migrations)

	// Initialize migration tables
	if err := migrator.Init(context.Background()); err != nil {
		logging.Logger.WithError(err).Fatal("failed to initialize migrator")
	}

	// Get status
	ms, err := migrator.MigrationsWithStatus(context.Background())
	if err != nil {
		logging.Logger.WithError(err).Fatal("failed to get migration status")
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

// LogMigration logs a migration message with the migration version.
// This provides consistent structured logging for all migrations.
func LogMigration(version, msg string) {
	if logging.Logger != nil {
		logging.Logger.WithField("migration", version).Info(msg)
	}
}

// LogMigrationError logs a migration error with the migration version.
// This provides consistent structured logging for migration errors.
func LogMigrationError(version string, msg string, err error) {
	if logging.Logger != nil {
		logging.Logger.WithField("migration", version).WithError(err).Error(msg)
	}
}

// Reset drops all tables and re-runs all migrations
// CAUTION: This will delete all data
func Reset() {
	// First reset the database by dropping all tables
	err := ResetDatabase()
	if err != nil {
		logging.Logger.WithError(err).Fatal("failed to reset database")
	}

	// Then run all migrations
	db, err := database.DBConn()
	if err != nil {
		logging.Logger.WithError(err).Fatal("failed to connect to database")
	}
	defer func() { _ = db.Close() }()

	// Initialize new migrator
	migrator := migrate.NewMigrator(db, Migrations)

	if err := migrator.Init(context.Background()); err != nil {
		logging.Logger.WithError(err).Fatal("failed to initialize migrator")
	}

	// Run migrations
	fmt.Println("Running all migrations...")
	group, err := migrator.Migrate(context.Background())
	if err != nil {
		logging.Logger.WithError(err).Fatal("migration failed")
	}

	fmt.Printf("Database reset and migration completed successfully. Migrated to %s\n", group)
}
