package cmd

import (
	"github.com/moto-nrw/project-phoenix/database/migrations"
	"github.com/spf13/cobra"
)

// fixConstraintsCmd represents the command to fix constraints in the database
var fixConstraintsCmd = &cobra.Command{
	Use:   "fix-constraints",
	Short: "fix database constraints to match ER diagram",
	Long:  `Applies all constraints needed to match the ER diagram exactly`,
	Run: func(cmd *cobra.Command, args []string) {
		migrations.FixConstraints()
	},
}

func init() {
	RootCmd.AddCommand(fixConstraintsCmd)
}
