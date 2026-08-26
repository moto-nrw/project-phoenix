package api

import (
	"context"
	"fmt"
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

type buildStateStep struct {
	seeder *Seeder
}

func (buildStateStep) Name() string { return "Writing seed state" }

func (s buildStateStep) Run(_ context.Context, rt *Runtime) error {
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
	state.Topology.Mode = "full-demo"
	state.Scenarios.DefaultPlayer = "pyreportal"
	state.Scenarios.DefaultMode = "hybrid"
	state.Parents = append([]ParentCredentials(nil), rt.Parents...)
	state.Credentials.Parents = append([]ParentCredentials(nil), rt.Parents...)
	state.Enrollment = cloneEnrollmentState(rt.Enrollment)
	state.Entities.Enrollment = cloneEnrollmentState(rt.Enrollment)
	state.Normalize()

	rt.State = state
	if err := WriteSeedState(state, DefaultSeedStatePath); err != nil {
		return err
	}
	fmt.Printf("Seed state written to %s\n", DefaultSeedStatePath)
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
	s.seeder.printSuccessSummary(rt.Bootstrap.AdminEmail, rt.Bootstrap.AdminPassword, rt.Result, rt.State)
	return nil
}

func fullDemoWorkflow(seeder *Seeder) Workflow {
	return Workflow{
		Name: "full-demo",
		Steps: []Step{
			healthCheckStep{},
			operatorLoginStep{},
			bootstrapTenantStep{seeder: seeder},
			seedMasterDataStep{seeder: seeder},
			markStudentsSickStep{},
			seedPrivacyConsentsStep{},
			seedCareExitsStep{},
			seedAnnouncementsStep{},
			seedFileStorageStep{},
			seedTimeTrackingHistoryStep{},
			parentEnrollmentSeedStep{seeder: seeder},
			buildStateStep{seeder: seeder},
			printSummaryStep{seeder: seeder},
		},
	}
}
