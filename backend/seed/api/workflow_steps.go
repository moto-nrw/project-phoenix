package api

import (
	"context"
	"fmt"

	contract "github.com/moto-nrw/project-phoenix/e2e/contract"
)

type healthCheckStep struct{}

func (healthCheckStep) Name() string { return "Server health check" }

func (healthCheckStep) Run(ctx context.Context, rt *Runtime) error {
	fmt.Printf("Connecting to %s...\n", rt.Client.baseURL)
	return rt.Adapter.CheckHealth(ctx)
}

type operatorLoginStep struct{}

func (operatorLoginStep) Name() string { return "Login" }

func (operatorLoginStep) Run(ctx context.Context, rt *Runtime) error {
	fmt.Printf("Logging in as operator %s...\n", rt.OperatorEmail)
	auth, err := rt.Adapter.LoginOperator(ctx, rt.OperatorEmail, rt.OperatorPassword)
	if err != nil {
		return err
	}
	rt.SetOperatorAuth(auth)
	fmt.Println("Operator authenticated")
	fmt.Println()
	return nil
}

type bootstrapTenantStep struct {
	seeder *Seeder
}

func (bootstrapTenantStep) Name() string { return "Tenant bootstrap" }

func (s bootstrapTenantStep) Run(ctx context.Context, rt *Runtime) error {
	bootstrapState, err := s.seeder.bootstrapTenant(ctx)
	if err != nil {
		return err
	}
	rt.Bootstrap = bootstrapState

	fmt.Printf("Logging in as invited school admin %s...\n", bootstrapState.AdminEmail)
	tenantAuth, err := rt.Adapter.LoginTenant(ctx, bootstrapState.AdminEmail, bootstrapState.AdminPassword, bootstrapState.TenantSlug)
	if err != nil {
		return err
	}
	rt.SetTenantAuth(tenantAuth)
	fmt.Println("School admin authenticated")
	fmt.Println()
	return nil
}

type seedMasterDataStep struct {
	seeder *Seeder
}

func (seedMasterDataStep) Name() string { return "Stammdaten seeding" }

func (s seedMasterDataStep) Run(ctx context.Context, rt *Runtime) error {
	fixedSeeder := NewFixedSeeder(rt.Client, rt.Verbose, s.seeder.options.StaffPassword)
	fixedResult, err := fixedSeeder.Seed(ctx)
	if err != nil {
		return err
	}

	rt.FixedSeeder = fixedSeeder
	rt.Result.Fixed = fixedResult
	fmt.Println()
	return nil
}

type markStudentsSickStep struct{}

func (markStudentsSickStep) Name() string { return "Marking students sick" }

func (markStudentsSickStep) Run(ctx context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil || rt.Result.Fixed == nil {
		return fmt.Errorf("fixed seeder result not available")
	}
	return rt.FixedSeeder.MarkStudentsSick(ctx, rt.Result.Fixed)
}

type buildSeedStateStep struct {
	seeder *Seeder
}

func (buildSeedStateStep) Name() string { return "Writing seed state" }

func (s buildSeedStateStep) Run(_ context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("fixed seeder not available")
	}

	state := s.seeder.collectSeedState(rt.FixedSeeder, rt.StaffPIN, rt.Bootstrap)
	state.Credentials.Operator = &SeedOperatorCredentials{
		Email:    rt.OperatorEmail,
		Password: rt.OperatorPassword,
	}
	state.Topology.Organizations = 1
	state.Topology.Schools = 1
	if rt.SecondTenant != nil {
		// Second school lives under the same organization, so the org
		// count stays 1 and only the school count bumps.
		state.Topology.Schools = 2
	}
	state.Topology.Mode = "full-demo"
	state.Scenarios.DefaultPlayer = "pyreportal"
	state.Scenarios.DefaultMode = "hybrid"
	state.Normalize()

	rt.State = state
	statePath := s.seeder.options.StatePath
	if statePath == "" {
		statePath = DefaultSeedStatePath
	}
	if err := WriteSeedState(state, statePath); err != nil {
		return err
	}
	fmt.Printf("Seed state written to %s\n", statePath)
	return nil
}

type writeE2EManifestStep struct {
	seeder *Seeder
}

func (writeE2EManifestStep) Name() string { return "Writing E2E manifest" }

func (s writeE2EManifestStep) Run(_ context.Context, rt *Runtime) error {
	manifest, err := s.seeder.buildE2EManifest(rt)
	if err != nil {
		return err
	}

	statePath := s.seeder.options.StatePath
	if statePath == "" {
		statePath = contract.ManifestPath
	}
	if err := contract.WriteManifest(manifest, statePath); err != nil {
		return err
	}
	fmt.Printf("E2E manifest written to %s\n", statePath)
	return nil
}

type writeSimulatorConfigStep struct{}

func (writeSimulatorConfigStep) Name() string { return "Generating simulator config" }

func (writeSimulatorConfigStep) Run(_ context.Context, rt *Runtime) error {
	if rt.State == nil {
		return fmt.Errorf("seed state not available")
	}
	simPath := "simulator/iot/simulator.yaml"
	if err := WriteSimulatorConfig(rt.State, simPath); err != nil {
		return err
	}
	fmt.Printf("Simulator config written to %s\n", simPath)
	fmt.Println()
	return nil
}

type printSummaryStep struct {
	seeder *Seeder
}

func (printSummaryStep) Name() string { return "Printing summary" }

func (s printSummaryStep) Run(_ context.Context, rt *Runtime) error {
	if rt.Bootstrap == nil {
		return fmt.Errorf("bootstrap state not available")
	}
	if rt.Result == nil {
		return fmt.Errorf("seed result not available")
	}
	s.seeder.printSuccessSummary(rt.Bootstrap.AdminEmail, rt.Bootstrap.AdminPassword, rt.Result)
	return nil
}

func fullDemoWorkflow(seeder *Seeder) Workflow {
	steps := []Step{
		healthCheckStep{},
		operatorLoginStep{},
		bootstrapTenantStep{seeder: seeder},
		seedMasterDataStep{seeder: seeder},
		markStudentsSickStep{},
		seedPrivacyConsentsStep{},
		seedAnnouncementsStep{},
		seedSuggestionsStep{},
	}

	// Second-tenant provisioning runs after the main demo data is in
	// place — it depends on the named LinkEmail account having been
	// created by the master-data step, on the operator being logged in
	// (so we can create the school + invite), and on roles existing in
	// the new tenant. Inserting it here keeps the scenario-specific output
	// step below as the single place where the canonical artifact is written.
	if seeder.options.SecondTenant != nil {
		steps = append(steps, secondTenantStep{seeder: seeder})
	}

	steps = append(steps, buildSeedStateStep{seeder: seeder})
	steps = append(steps, writeSimulatorConfigStep{})
	steps = append(steps, printSummaryStep{seeder: seeder})

	return Workflow{
		Name:  "full-demo",
		Steps: steps,
	}
}

func e2eWorkflow(seeder *Seeder) Workflow {
	steps := []Step{
		healthCheckStep{},
		operatorLoginStep{},
		bootstrapTenantStep{seeder: seeder},
		seedMasterDataStep{seeder: seeder},
		markStudentsSickStep{},
		seedPrivacyConsentsStep{},
		seedAnnouncementsStep{},
		seedSuggestionsStep{},
	}

	if seeder.options.SecondTenant != nil {
		steps = append(steps, secondTenantStep{seeder: seeder})
	}

	steps = append(steps, provisionE2ECheckinStep{seeder: seeder})
	steps = append(steps, writeE2EManifestStep{seeder: seeder})
	steps = append(steps, printSummaryStep{seeder: seeder})

	return Workflow{
		Name:  "e2e-prepare",
		Steps: steps,
	}
}
