package cmd

import (
	"context"
	"log"

	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
	"github.com/spf13/cobra"
)

// seedCmd represents the seed command
var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed the database with test data via API",
	Long: `Seed the database with demo data for development and testing.

Seeds via HTTP API calls against a running server. This ensures all data
passes through validation, middleware, and tenant resolution — exactly
like real users would interact with the system.

REQUIRES: A running server with an admin account already created
(via migrations or manual setup).

DEMO DATA:
- 12 rooms (OGS rooms, gym, schoolyard, cafeteria)
- 20 staff members (10 admin, 10 betreuer)
- 100 students (10 groups x 10)
- 10 activities (homework, sports, crafts, etc.)
- 10 IoT devices for RFID scanning

OUTPUT FILES:
- .seed-state.json — all created IDs, credentials, and API keys
- simulator.yaml   — ready-to-use simulator configuration

Usage:
  go run main.go seed --email admin@example.com --password 'Test1234%' --pin 1234
  go run main.go seed --email admin@example.com --password 'Test1234%' --pin 1234 --verbose
  go run main.go seed --email admin@example.com --password 'Test1234%' --pin 1234 --url http://localhost:8080`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		pin, _ := cmd.Flags().GetString("pin")
		url, _ := cmd.Flags().GetString("url")
		verbose, _ := cmd.Flags().GetBool("verbose")

		if email == "" || password == "" || pin == "" {
			log.Fatal("--email, --password, and --pin are required")
		}

		seeder := seedapi.NewSeeder(url, verbose)

		result, err := seeder.Seed(ctx, email, password, pin)
		if err != nil {
			log.Fatal(err)
		}

		_ = result
	},
}

func init() {
	RootCmd.AddCommand(seedCmd)
	seedCmd.Flags().String("email", "", "Admin email for API authentication (required)")
	seedCmd.Flags().String("password", "", "Admin password for API authentication (required)")
	seedCmd.Flags().String("pin", "", "Staff PIN for IoT authentication (required)")
	seedCmd.Flags().String("url", "http://localhost:8080", "Backend API URL")
	seedCmd.Flags().Bool("verbose", false, "Enable verbose logging")
}
