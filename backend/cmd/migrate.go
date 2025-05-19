package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/migrations"
	"github.com/spf13/cobra"
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "use bun migration tool",
	Long:  `run bun migrations`,
	Run: func(cmd *cobra.Command, args []string) {
		migrations.Migrate()
	},
}

// migrateResetCmd represents the migrate reset command
var migrateResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "reset database and run all migrations",
	Long:  `WARNING: This will delete all data in the database and run all migrations from scratch`,
	Run: func(cmd *cobra.Command, args []string) {
		migrations.Reset()
	},
}

// migrateStatusCmd represents the migrate status command
var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "show migration status",
	Long:  `Display the status of all migrations, showing which ones have been applied`,
	Run: func(cmd *cobra.Command, args []string) {
		migrations.MigrateStatus()
	},
}

// migrateValidateCmd represents the migrate validate command
var migrateValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "validate migration dependencies",
	Long:  `Check all migration dependencies for correctness and ordering`,
	Run: func(cmd *cobra.Command, args []string) {
		// Connect to database
		db, err := database.DBConn()
		if err != nil {
			log.Fatal(err)
		}
		defer func() { _ = db.Close() }()

		// Validate migrations
		ctx := context.Background()
		err = migrations.ValidateMigrations(ctx, db)
		if err != nil {
			fmt.Printf("Migration validation failed: %v\n", err)
			return
		}

		fmt.Println("All migrations validated successfully!")
		migrations.PrintMigrationPlan()
	},
}

func init() {
	RootCmd.AddCommand(migrateCmd)
	migrateCmd.AddCommand(migrateResetCmd)
	migrateCmd.AddCommand(migrateStatusCmd)
	migrateCmd.AddCommand(migrateValidateCmd)
}
