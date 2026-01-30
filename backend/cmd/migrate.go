package cmd

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/migrations"
	"github.com/spf13/cobra"
)

// Migration function variables for testability (overridden in tests).
var (
	migrateFn       = migrations.Migrate
	migrateResetFn  = migrations.Reset
	migrateStatusFn = migrations.MigrateStatus
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "use bun migration tool",
	Long:  `run bun migrations`,
	Run: func(cmd *cobra.Command, args []string) {
		migrateFn()
	},
}

// migrateResetCmd represents the migrate reset command
var migrateResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "reset database and run all migrations",
	Long:  `WARNING: This will delete all data in the database and run all migrations from scratch`,
	Run: func(cmd *cobra.Command, args []string) {
		migrateResetFn()
	},
}

// migrateStatusCmd represents the migrate status command
var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "show migration status",
	Long:  `Display the status of all migrations, showing which ones have been applied`,
	Run: func(cmd *cobra.Command, args []string) {
		migrateStatusFn()
	},
}

// migrateValidateCmd represents the migrate validate command
var migrateValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "validate migration dependencies",
	Long:  `Check all migration dependencies for correctness and ordering`,
	RunE: func(cmd *cobra.Command, args []string) error {
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
