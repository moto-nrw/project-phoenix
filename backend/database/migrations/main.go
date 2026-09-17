package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// newMigrator builds the migrator every entry point here uses, with the
// per-migration log hooks already wired in.
func newMigrator(db *bun.DB, runLog *runnerLog) *migrate.Migrator {
	options := []migrate.MigratorOption{migrate.WithMarkAppliedOnSuccess(true)}
	if runLog != nil {
		options = append(options, runLog.migratorOptions()...)
	}
	return migrate.NewMigrator(db, Migrations, options...)
}

// Migrate runs all pending migrations
func Migrate(ctx context.Context, db *bun.DB) error {
	return migrateWithLogger(ctx, db, slog.Default())
}

func migrateWithLogger(ctx context.Context, db *bun.DB, logger *slog.Logger) error {
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	runLog := newRunnerLog(migrationLogger(logger))
	migrator := newMigrator(db, runLog)

	// Initialize migration tables
	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("initialize migrations: %w", err)
	}

	// Validate migrations before running
	if err := ValidateMigrations(); err != nil {
		return fmt.Errorf("validate migrations: %w", err)
	}

	// Report the pending migrations, not the full plan. Compose and the native
	// dev start run `migrate` before every server start, so printing all ~500
	// planned migrations buried each startup log even when nothing was pending
	// (#3300). `migrate validate` still prints the full plan.
	status, err := migrator.MigrationsWithStatus(ctx)
	if err != nil {
		return fmt.Errorf("load migration status: %w", err)
	}
	pending := status.Unapplied()
	if len(pending) == 0 {
		runLog.logger.InfoContext(ctx, "no pending migrations", "applied", len(status))
		return nil
	}
	firstVersion, lastVersion := pendingVersionRange(pending)
	runLog.logger.InfoContext(ctx, "pending migrations",
		"count", len(pending),
		"first_version", firstVersion,
		"last_version", lastVersion,
	)

	// Run migrations
	group, err := migrator.Migrate(ctx)
	if err != nil {
		runLog.logFailure(ctx, err)
		return fmt.Errorf("run migrations: %w", err)
	}

	runLog.logger.InfoContext(ctx, "migrations applied",
		"count", len(group.Migrations),
		"group_id", group.ID,
	)
	return nil
}

// MigrateStatus shows current migration status
func MigrateStatus(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	migrator := newMigrator(db, nil)

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

		// Resolve bun's name to the registry's version and description. Looking
		// the registry up by m.Name never matched before #3300: the registry is
		// keyed by semantic version, m.Name is the filename prefix, so every
		// line printed an empty description.
		version, description := migrationIdentity(&m)
		if description != "" {
			description = fmt.Sprintf(" - %s", description)
		}

		fmt.Printf("V%s (%s): %s%s\n", version, m.Name, status, description)
	}
	return nil
}

// Reset drops all tables and re-runs all migrations
// CAUTION: This will delete all data
func Reset(ctx context.Context, db *bun.DB) error {
	return resetWithLogger(ctx, db, slog.Default())
}

func resetWithLogger(ctx context.Context, db *bun.DB, logger *slog.Logger) error {
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	runLog := newRunnerLog(migrationLogger(logger))

	// First reset the database by dropping all tables
	if err := resetDatabase(ctx, db, runLog.logger); err != nil {
		return fmt.Errorf("reset database: %w", err)
	}

	// Initialize new migrator
	migrator := newMigrator(db, runLog)

	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("initialize migrations after reset: %w", err)
	}

	// Run migrations
	runLog.logger.InfoContext(ctx, "replaying all migrations", "count", len(Migrations.Sorted()))
	group, err := migrator.Migrate(ctx)
	if err != nil {
		runLog.logFailure(ctx, err)
		return fmt.Errorf("run migrations after reset: %w", err)
	}

	runLog.logger.InfoContext(ctx, "database reset complete",
		"count", len(group.Migrations),
		"group_id", group.ID,
	)
	return nil
}
