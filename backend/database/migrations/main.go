package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// Migrate runs all pending migrations
func Migrate(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	migrator := migrate.NewMigrator(db, Migrations, migrate.WithMarkAppliedOnSuccess(true))

	// Initialize migration tables
	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("initialize migrations: %w", err)
	}

	// Validate migrations before running
	if err := ValidateMigrations(); err != nil {
		return fmt.Errorf("validate migrations: %w", err)
	}

	// Print migration plan
	if err := PrintMigrationPlan(); err != nil {
		return fmt.Errorf("print migration plan: %w", err)
	}

	// Run migrations
	group, err := migrator.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	if group.ID == 0 {
		fmt.Println("No new migrations to run")
	} else {
		fmt.Printf("Migrated to %s\n", group)
	}
	return nil
}

// MigrateStatus shows current migration status
func MigrateStatus(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	migrator := migrate.NewMigrator(db, Migrations, migrate.WithMarkAppliedOnSuccess(true))

	// Initialize migration tables
	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("initialize migrations: %w", err)
	}

	// Get status
	ms, err := migrator.MigrationsWithStatus(ctx)
	if err != nil {
		return fmt.Errorf("load migration status: %w", err)
	}

	fmt.Println("Migration Status:")
	fmt.Println("=================")

	if len(ms) == 0 {
		fmt.Println("No migrations found")
		return nil
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
	return nil
}

// Reset drops all tables and re-runs all migrations
// CAUTION: This will delete all data
func Reset(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	// First reset the database by dropping all tables
	err := ResetDatabase(ctx, db)
	if err != nil {
		return fmt.Errorf("reset database: %w", err)
	}

	// Initialize new migrator
	migrator := migrate.NewMigrator(db, Migrations, migrate.WithMarkAppliedOnSuccess(true))

	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("initialize migrations after reset: %w", err)
	}

	// Run migrations
	fmt.Println("Running all migrations...")
	group, err := migrator.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("run migrations after reset: %w", err)
	}

	fmt.Printf("Database reset and migration completed successfully. Migrated to %s\n", group)
	return nil
}
