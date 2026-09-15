package api

import (
	"context"
	"fmt"
	"slices"
	"sort"
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

type seedFamilyProtectionStep struct{}

func (seedFamilyProtectionStep) Name() string { return "Seeding family protection" }

func (seedFamilyProtectionStep) Run(_ context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("fixed seeder not available")
	}
	studentID, ok := rt.FixedSeeder.studentIDByIndex[1]
	if !ok {
		return fmt.Errorf("family protection demo student not available")
	}
	_, err := rt.Client.Put(fmt.Sprintf("/api/students/%d/family-protection", studentID), map[string]any{
		"enabled": true,
		"reason":  "Demo: private Familienangaben schützen",
	})
	if err != nil {
		return fmt.Errorf("seed family protection: %w", err)
	}
	fmt.Println("  1 family protection rule created")
	return nil
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
	if virtual, ok := rt.Values["profile.virtual_device"].(SeedDevice); ok {
		state.Devices[virtual.DeviceID] = virtual
	}
	state.Credentials.Operator = &SeedOperatorCredentials{
		Email:    rt.OperatorEmail,
		Password: rt.OperatorPassword,
	}
	state.Topology.Organizations = 1
	state.Topology.Schools = 1 + len(rt.AdditionalProfiles)

	state.Topology.Mode = "profiles"
	state.Scenarios.DefaultPlayer = "pyreportal"
	state.Scenarios.DefaultMode = "hybrid"
	state.Parents = append([]ParentCredentials(nil), rt.Parents...)
	state.Credentials.Parents = append([]ParentCredentials(nil), rt.Parents...)
	state.Enrollment = cloneEnrollmentState(rt.Enrollment)
	state.Entities.Enrollment = cloneEnrollmentState(rt.Enrollment)
	state.Normalize()
	mergeAdditionalProfiles(state, rt.AdditionalProfiles)

	for _, key := range []string{enrollmentWeeklyProfileKey, enrollmentBookingsProfileKey} {
		if additional, ok := rt.Values[key].(*SeedState); ok {
			mergeAdditionalProfiles(state, additional.Profiles)
		}
	}
	state.Topology.Organizations = len(state.Organizations)
	state.Topology.Schools = len(state.Profiles)

	rt.State = state
	if err := WriteSeedState(state, s.seeder.statePath); err != nil {
		return err
	}
	fmt.Printf("Seed state written to %s\n", s.seeder.statePath)
	return nil
}

func mergeAdditionalProfiles(state *SeedState, profiles map[string]*SeedProfile) {
	for key, profile := range profiles {
		if profile == nil {
			continue
		}
		state.Profiles[key] = profile
		organization := state.Organizations[profile.Organization.Slug]
		organization.ID = profile.Organization.ID
		organization.Name = profile.Organization.Name
		organization.Slug = profile.Organization.Slug
		organization.Profiles = append(organization.Profiles, key)
		sort.Strings(organization.Profiles)
		organization.Profiles = slices.Compact(organization.Profiles)
		state.Organizations[organization.Slug] = organization
	}
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
			configureProfileStep{definition: seeder.definition},
			seedMasterDataStep{seeder: seeder},
			seedPlanningDemoStep{},
			seedStudentStatusVariantsStep{},
			seedOperationsDemoStep{},
			seedHomeLayoutStep{},
			seedStaffMasterDataStep{},
			seedImportAuditStep{},
			seedAuditLifecycleStep{},
			seedPrivacyConsentsStep{},
			seedFamilyProtectionStep{},
			markStudentsSickStep{},
			seedCareExitsStep{},
			seedAnnouncementsStep{},
			seedStaffMessagingStep{},
			seedStaffNoticesStep{},
			seedFileStorageStep{},
			// Vor der App-Historie: der IoT-Sitzungsstart erzeugt den echten
			// NFC-Arbeitsblock. Nach einem App-Checkout am selben Tag verhindert
			// die Zeiterfassung bewusst einen erneuten Auto-Check-in.
			seedStatisticsDemoStep{},
			seedTimeTrackingHistoryStep{},
			seedDataAccessAuditStep{},
			// Rührt weder an der Zeiterfassung noch am NFC-Block: legt nur
			// vergangene Kurstermine samt Anwesenheit an (#2891).
			seedCourseParticipationStep{},
			parentEnrollmentSeedStep{seeder: seeder},
			seedParentEngagementStep{},
			seedGradeTransitionStep{},
			seedParentLetterStep{},
			seedInactiveAccountStep{},
			verifyProfileStep{definition: seeder.definition},
			manualProfileStep{seeder: seeder},
			seedEnrollmentWeeklyProfileStep{seeder: seeder},
			seedEnrollmentBookingsProfileStep{seeder: seeder},
			buildStateStep{seeder: seeder},
			printSummaryStep{seeder: seeder},
		},
	}
}
