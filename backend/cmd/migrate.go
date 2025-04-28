package cmd

import (
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

// This command has been moved to addsampledata.go

func init() {
	RootCmd.AddCommand(migrateCmd)
	migrateCmd.AddCommand(migrateResetCmd)
}
