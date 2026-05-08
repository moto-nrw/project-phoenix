package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/moto-nrw/project-phoenix/database/migrations"
	contract "github.com/moto-nrw/project-phoenix/e2e/contract"
	"github.com/moto-nrw/project-phoenix/e2e/scenarios"
	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
)

const (
	DefaultSeedURL = "http://localhost:8080"
)

type PrepareOptions struct {
	Scenario string
	Verbose  bool
}

func (o *PrepareOptions) normalize() {
	if o.Scenario == "" {
		o.Scenario = scenarios.DefaultPrepareScenario().Name
	}
}

// Prepare rebuilds the canonical multi-tenant E2E world and writes the single
// manifest contract consumed by Playwright.
func Prepare(ctx context.Context, options PrepareOptions) error {
	options.normalize()
	definition, ok := scenarios.Lookup(options.Scenario)
	if !ok {
		return fmt.Errorf("unknown e2e scenario %q", options.Scenario)
	}
	identity := definition.Identity
	configureOperatorEnv(identity)

	runtime, err := canonicalRuntime()
	if err != nil {
		return fmt.Errorf("canonical e2e runtime: %w", err)
	}

	migrations.Reset()

	seeder := seedapi.NewSeeder(DefaultSeedURL, options.Verbose, seedapi.SeedOptions{
		Scenario:   options.Scenario,
		E2ERuntime: runtime,
	})
	if _, err := seeder.PrepareE2E(ctx, identity.OperatorEmail, identity.OperatorPassword, identity.StaffPIN); err != nil {
		return fmt.Errorf("seed e2e scenario %q: %w", options.Scenario, err)
	}

	fmt.Printf("Canonical E2E world is ready. Manifest written to %s\n", contract.ManifestPath)
	return nil
}

func configureOperatorEnv(identity scenarios.Identity) {
	_ = os.Setenv("OPERATOR_EMAIL", identity.OperatorEmail)
	_ = os.Setenv("OPERATOR_PASSWORD", identity.OperatorPassword)
	_ = os.Setenv("OPERATOR_DISPLAY_NAME", identity.OperatorDisplayName)
}
