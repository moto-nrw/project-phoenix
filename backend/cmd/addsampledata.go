package cmd

import (
	"github.com/moto-nrw/project-phoenix/database/migrations"
	"github.com/spf13/cobra"
)

// addSampleDataCmd represents the command to add sample data
var addSampleDataCmd = &cobra.Command{
	Use:   "add-sample-data",
	Short: "add sample data to the database",
	Long:  `Create tables and add sample student data for testing`,
	Run: func(cmd *cobra.Command, args []string) {
		migrations.AddSampleStudents()
	},
}

func init() {
	RootCmd.AddCommand(addSampleDataCmd)
}
