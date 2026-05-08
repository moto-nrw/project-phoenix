package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/moto-nrw/project-phoenix/database/migrations"
	contract "github.com/moto-nrw/project-phoenix/e2e/contract"
	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
)

const (
	DefaultSeedURL = "http://localhost:8080"
)

type PrepareOptions struct {
	URL     string
	Verbose bool
}

func (o *PrepareOptions) normalize() {
	if o.URL == "" {
		o.URL = DefaultSeedURL
	}
}

// Prepare rebuilds the canonical multi-tenant E2E world and writes the single
// manifest contract consumed by Playwright.
func Prepare(ctx context.Context, options PrepareOptions) error {
	options.normalize()
	identity := seedapi.CanonicalE2EIdentity()
	configureOperatorEnv(identity)

	migrations.Reset()

	seeder := seedapi.NewSeeder(options.URL, options.Verbose, seedapi.SeedOptions{
		Scenario: seedapi.SeedScenarioE2E,
	})
	if _, err := seeder.Seed(ctx, identity.OperatorEmail, identity.OperatorPassword, identity.StaffPIN); err != nil {
		return fmt.Errorf("seed canonical e2e scenario: %w", err)
	}

	fmt.Printf("Canonical E2E world is ready. Manifest written to %s\n", contract.ManifestPath)
	return nil
}

func configureOperatorEnv(identity seedapi.E2EIdentity) {
	_ = os.Setenv("OPERATOR_EMAIL", identity.OperatorEmail)
	_ = os.Setenv("OPERATOR_PASSWORD", identity.OperatorPassword)
	_ = os.Setenv("OPERATOR_DISPLAY_NAME", identity.OperatorDisplayName)
}
