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

OPTIONAL FLAGS (deterministic mode):
By default, the seeder generates random suffixes and passwords for each run.
Use optional flags to get deterministic, memorable credentials:

  --tenant-slug demo-school    Fixed tenant slug (requires 'migrate reset' before re-seeding)
  --staff-password 'Test1234%' Shared password for all 20 staff accounts
  --admin-email admin@test.com Fixed email for the bootstrap school admin

Usage:
  go run main.go seed --email op@example.com --password 'Test1234%' --pin 1234
  go run main.go seed --email op@example.com --password 'Test1234%' --pin 1234 --tenant-slug demo-school --staff-password 'Test1234%' --admin-email school-admin@example.com`,
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

		tenantSlug, _ := cmd.Flags().GetString("tenant-slug")
		staffPassword, _ := cmd.Flags().GetString("staff-password")
		adminEmail, _ := cmd.Flags().GetString("admin-email")
		statePath, _ := cmd.Flags().GetString("state-path")

		options := seedapi.SeedOptions{
			TenantSlug:    tenantSlug,
			StaffPassword: staffPassword,
			AdminEmail:    adminEmail,
			StatePath:     statePath,
		}

		withSecondTenant, _ := cmd.Flags().GetBool("with-second-tenant")
		if withSecondTenant {
			secondSlug, _ := cmd.Flags().GetString("second-tenant-slug")
			secondName, _ := cmd.Flags().GetString("second-tenant-name")
			secondAdminEmail, _ := cmd.Flags().GetString("second-tenant-admin-email")
			secondAdminPassword, _ := cmd.Flags().GetString("second-tenant-admin-password")
			secondLinkEmail, _ := cmd.Flags().GetString("second-tenant-link-email")

			// Defaults match the values the E2E suite expects via
			// frontend/e2e/helpers/seed-state.ts. Devs can override
			// each field individually for non-E2E experiments.
			if secondSlug == "" {
				secondSlug = "second-school"
			}
			if secondName == "" {
				secondName = "Demo School 2"
			}
			if secondAdminEmail == "" {
				secondAdminEmail = "admin-b@e2e.local"
			}
			if secondAdminPassword == "" {
				if staffPassword != "" {
					secondAdminPassword = staffPassword
				} else {
					log.Fatal("--with-second-tenant requires --second-tenant-admin-password (or --staff-password as a default)")
				}
			}
			if secondLinkEmail == "" {
				// demo1 is the OGS-Büro admin from the main seed; this
				// is the account whose access matrix the tenant-switch
				// spec depends on.
				secondLinkEmail = "demo1@mail.de"
			}

			options.SecondTenant = &seedapi.SecondTenantOptions{
				Slug:          secondSlug,
				Name:          secondName,
				AdminEmail:    secondAdminEmail,
				AdminPassword: secondAdminPassword,
				LinkEmail:     secondLinkEmail,
			}
		}

		seeder := seedapi.NewSeeder(url, verbose, options)
		result, err := seeder.Seed(ctx, email, password, pin)
		if err != nil {
			log.Fatal(err)
		}

		_ = result
	},
}

func init() {
	RootCmd.AddCommand(seedCmd)
	seedCmd.Flags().String("email", "", "Operator email for API authentication (required)")
	seedCmd.Flags().String("password", "", "Operator password for API authentication (required)")
	seedCmd.Flags().String("pin", "", "Staff PIN for IoT authentication (required)")
	seedCmd.Flags().String("url", "http://localhost:8080", "Backend API URL")
	seedCmd.Flags().Bool("verbose", false, "Enable verbose logging")
	seedCmd.Flags().String("tenant-slug", "", "Fixed tenant slug (deterministic mode, requires migrate reset before re-seeding)")
	seedCmd.Flags().String("staff-password", "", "Shared password for all 20 staff accounts (deterministic mode)")
	seedCmd.Flags().String("admin-email", "", "Fixed email for bootstrap school admin (deterministic mode)")
	seedCmd.Flags().String("state-path", seedapi.DefaultSeedStatePath, "Path to write the seed state JSON (E2E uses .seed-state-e2e.json to avoid colliding with dev seed)")

	// Second-tenant provisioning: opt-in extension used by the E2E suite
	// so the TenantSwitcher dropdown has >=2 mappings to render. Off by
	// default — regular dev seeds stay single-tenant.
	seedCmd.Flags().Bool("with-second-tenant", false, "Provision a sibling school under the same organization and link the named account into both (E2E tenant-switch coverage)")
	seedCmd.Flags().String("second-tenant-slug", "", "Slug for the second school (default: second-school)")
	seedCmd.Flags().String("second-tenant-name", "", "Display name for the second school (default: Demo School 2)")
	seedCmd.Flags().String("second-tenant-admin-email", "", "Email for the second-school admin (default: admin-b@e2e.local)")
	seedCmd.Flags().String("second-tenant-admin-password", "", "Password for the second-school admin (defaults to --staff-password if set)")
	seedCmd.Flags().String("second-tenant-link-email", "", "Account email to link into both tenants (default: demo1@mail.de)")
}
