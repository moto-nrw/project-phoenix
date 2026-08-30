package cmd

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/migrations"
	"github.com/spf13/cobra"
	"github.com/uptrace/bun"
)

type migrationOperation func(context.Context, *bun.DB) error

type migrateRoot struct {
	openDatabase func() (*bun.DB, error)
	migrate      migrationOperation
	reset        migrationOperation
	status       migrationOperation
}

func (root migrateRoot) operation(command string) (migrationOperation, error) {
	switch command {
	case "migrate":
		return root.migrate, nil
	case "reset":
		return root.reset, nil
	case "status":
		return root.status, nil
	default:
		return nil, fmt.Errorf("unknown migration operation %q", command)
	}
}

func (root migrateRoot) runCommand(ctx context.Context, command string) error {
	operation, err := root.operation(command)
	if err != nil {
		return err
	}
	return root.run(ctx, operation)
}

func (root migrateRoot) run(ctx context.Context, operation migrationOperation) error {
	if root.openDatabase == nil {
		return fmt.Errorf("database opener is required")
	}
	if operation == nil {
		return fmt.Errorf("migration operation is required")
	}
	db, err := root.openDatabase()
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	if db == nil {
		return fmt.Errorf("database opener returned nil")
	}
	defer func() { _ = db.Close() }()
	return operation(ctx, db)
}

var defaultMigrateRoot = migrateRoot{
	openDatabase: database.DBConn,
	migrate:      migrations.Migrate,
	reset:        migrations.Reset,
	status:       migrations.MigrateStatus,
}

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "use bun migration tool",
	Long:  `run bun migrations`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return defaultMigrateRoot.runCommand(cmd.Context(), "migrate")
	},
}

// migrateResetCmd represents the migrate reset command
var migrateResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "reset database and run all migrations",
	Long:  `WARNING: This will delete all data in the database and run all migrations from scratch`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return defaultMigrateRoot.runCommand(cmd.Context(), "reset")
	},
}

// migrateStatusCmd represents the migrate status command
var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "show migration status",
	Long:  `Display the status of all migrations, showing which ones have been applied`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return defaultMigrateRoot.runCommand(cmd.Context(), "status")
	},
}

// migrateValidateCmd represents the migrate validate command
var migrateValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "validate migration dependencies",
	Long:  `Check all migration dependencies for correctness and ordering`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Detect duplicate version registrations by scanning source files.
		// In Docker, source files don't exist (only the compiled binary),
		// so DetectVersionCollisions gracefully returns nil when the directory is missing.
		if err := migrations.DetectVersionCollisions("database/migrations"); err != nil {
			return fmt.Errorf("migration version collision check failed: %w", err)
		}

		if err := migrations.ValidateMigrations(); err != nil {
			return fmt.Errorf("migration validation failed: %w", err)
		}

		fmt.Println("All migrations validated successfully!")
		migrations.PrintMigrationPlan()
		return nil
	},
}

func init() {
	RootCmd.AddCommand(migrateCmd)
	migrateCmd.AddCommand(migrateResetCmd)
	migrateCmd.AddCommand(migrateStatusCmd)
	migrateCmd.AddCommand(migrateValidateCmd)
}
