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

REQUIRES: A running server with an operator account already created.

DEMO DATA:
- 12 rooms (OGS rooms, gym, schoolyard, cafeteria)
- 20 staff members (10 admin, 10 betreuer)
- 100 students (10 groups x 10)
- 10 activities (homework, sports, crafts, etc.)
- 10 IoT devices for RFID scanning

OUTPUT FILES:
- .seed-state.json — all created IDs, credentials, and API keys
- simulator.yaml   — ready-to-use simulator configuration

CANONICAL E2E HARNESS:
The deterministic multi-tenant Playwright world does NOT live behind this
command. Use:

  go run main.go e2e prepare --scenario e2e-multi-tenant

That scenario path owns its tenant topology, credentials, fixtures, and
machine-readable contract. This seed command remains the developer-facing
"full demo world" bootstrap.

Usage:
  go run main.go seed --email op@example.com --password 'Test1234%' --pin 1234
  go run main.go seed --email op@example.com --password 'Test1234%' --pin 1234`,
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

		if err := assertNonProductionURL(url); err != nil {
			log.Fatal(err)
		}

		options := seedOptionsFromFlags(cmd)

		seeder := seedapi.NewSeeder(url, verbose, options)
		result, err := seeder.Seed(ctx, email, password, pin)
		if err != nil {
			log.Fatal(err)
		}

		_ = result
	},
}

func seedOptionsFromFlags(cmd *cobra.Command) seedapi.SeedOptions {
	tenantSlug, _ := cmd.Flags().GetString("tenant-slug")
	staffPassword, _ := cmd.Flags().GetString("staff-password")
	adminEmail, _ := cmd.Flags().GetString("admin-email")
	statePath, _ := cmd.Flags().GetString("state-path")

	return seedapi.SeedOptions{
		TenantSlug:    tenantSlug,
		StaffPassword: staffPassword,
		AdminEmail:    adminEmail,
		StatePath:     statePath,
	}
}

func init() {
	RootCmd.AddCommand(seedCmd)
	seedCmd.Flags().String("email", "", "Operator email for API authentication (required)")
	seedCmd.Flags().String("password", "", "Operator password for API authentication (required)")
	seedCmd.Flags().String("pin", "", "Staff PIN for IoT authentication (required)")
	seedCmd.Flags().String("url", "http://localhost:8080", "Backend API URL")
	seedCmd.Flags().Bool("verbose", false, "Enable verbose logging")
	seedCmd.Flags().String("tenant-slug", "", "Fixed tenant slug (deterministic mode; requires migrate reset before re-seeding)")
	seedCmd.Flags().String("staff-password", "", "Shared password for all 20 staff accounts (deterministic mode)")
	seedCmd.Flags().String("admin-email", "", "Fixed email for the bootstrap school admin (deterministic mode)")
	seedCmd.Flags().String("state-path", seedapi.DefaultSeedStatePath, "Path to write the seed state JSON")
	_ = seedCmd.Flags().MarkHidden("tenant-slug")
	_ = seedCmd.Flags().MarkHidden("staff-password")
	_ = seedCmd.Flags().MarkHidden("admin-email")
}
