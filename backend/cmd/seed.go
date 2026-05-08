package cmd

import (
	"context"
	"fmt"
	"log"
	"strings"

	contract "github.com/moto-nrw/project-phoenix/e2e/contract"
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

E2E SCENARIO OUTPUT:
- .e2e-manifest.json — dedicated Playwright manifest (actors, tenant topology,
                       auth/setup fixtures). Written only by --scenario e2e-multi-tenant.

OPTIONAL FLAGS (deterministic mode):
By default, the seeder generates random suffixes and passwords for each run.
Use optional flags to get deterministic, memorable credentials:

  --scenario e2e-multi-tenant  Canonical Playwright world (multi-tenant,
                               deterministic actors, manifest-friendly output)
  --tenant-slug demo-school    Fixed tenant slug (requires 'migrate reset' before re-seeding)
  --staff-password 'Test1234%' Shared password for all 20 staff accounts
  --admin-email admin@test.com Fixed email for the bootstrap school admin

When using --scenario e2e-multi-tenant, do not mix in the deterministic override flags
above. The scenario is the contract for the harness; if E2E needs a new
actor or fixture, extend backend/seed/api/scenarios.go instead of composing
ad hoc CLI overrides.

Usage:
  go run main.go seed --email op@example.com --password 'Test1234%' --pin 1234
  go run main.go seed --email op@example.com --password 'Test1234%' --pin 1234 --scenario e2e-multi-tenant
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
		if err := validateSeedFlagCombination(cmd); err != nil {
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

func validateSeedFlagCombination(cmd *cobra.Command) error {
	scenario, _ := cmd.Flags().GetString("scenario")
	if scenario != seedapi.SeedScenarioE2E {
		return nil
	}

	var overrides []string
	for _, flag := range []string{"tenant-slug", "staff-password", "admin-email"} {
		if cmd.Flags().Changed(flag) {
			overrides = append(overrides, "--"+flag)
		}
	}

	if len(overrides) == 0 {
		return nil
	}

	return fmt.Errorf(
		"%s cannot be combined with --scenario %s. The canonical E2E world is owned by backend/seed/api/scenarios.go; extend the scenario there instead of overriding it via CLI flags",
		strings.Join(overrides, ", "),
		seedapi.SeedScenarioE2E,
	)
}

func seedOptionsFromFlags(cmd *cobra.Command) seedapi.SeedOptions {
	tenantSlug, _ := cmd.Flags().GetString("tenant-slug")
	staffPassword, _ := cmd.Flags().GetString("staff-password")
	adminEmail, _ := cmd.Flags().GetString("admin-email")
	statePath, _ := cmd.Flags().GetString("state-path")
	scenario, _ := cmd.Flags().GetString("scenario")

	// Cobra returns the flag's default even when the caller omitted it.
	// Keep the zero value so scenario defaults can supply their own
	// manifest path, while an explicit --state-path still wins.
	if !cmd.Flags().Changed("state-path") {
		statePath = ""
	}

	return seedapi.SeedOptions{
		Scenario:      scenario,
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
	seedCmd.Flags().String("scenario", "", "Named seed scenario (e.g. e2e-multi-tenant)")
	seedCmd.Flags().String("tenant-slug", "", "Fixed tenant slug (deterministic mode; not for --scenario e2e-multi-tenant, requires migrate reset before re-seeding)")
	seedCmd.Flags().String("staff-password", "", "Shared password for all 20 staff accounts (deterministic mode; not for --scenario e2e-multi-tenant)")
	seedCmd.Flags().String("admin-email", "", "Fixed email for bootstrap school admin (deterministic mode; not for --scenario e2e-multi-tenant)")
	seedCmd.Flags().String("state-path", seedapi.DefaultSeedStatePath, "Path to write the output JSON (seed state by default; --scenario e2e-multi-tenant writes the dedicated "+contract.ManifestPath+" manifest)")
}
