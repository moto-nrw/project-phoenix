package api

import (
	"context"
	"fmt"
)

// seedStaffOnboardingStep sets the first steps of the demo school's staff
// (#3748), through the public routes and each under the person's own login:
// the first has finished one tour and skipped another, so the checklist shows
// all three states; everybody else has hidden it, as the team of a running
// school has. The product screenshots and the demo show those people, and the
// floating checklist would cover their screens.
type seedStaffOnboardingStep struct{}

func (seedStaffOnboardingStep) Name() string { return "Seeding staff first steps" }

func (seedStaffOnboardingStep) Run(_ context.Context, rt *Runtime) error {
	// rt.State is only assembled by buildStateStep at the very end of the
	// workflow, so the credentials come from the fixed seeder, like the
	// Team-Chat seed.
	if rt == nil || rt.Client == nil || rt.FixedSeeder == nil {
		return fmt.Errorf("staff first steps seed prerequisites not available")
	}
	staff, _ := buildStaffOrder(rt.FixedSeeder)
	if len(staff) < 2 {
		return fmt.Errorf("staff first steps require at least two staff accounts, got %d", len(staff))
	}
	defer rt.Client.BindAuth(rt.TenantAuth)

	started := staff[0]
	if err := rt.Client.Login(started.Email, started.Password); err != nil {
		return fmt.Errorf("login as staff %s: %w", started.Email, err)
	}
	for step, state := range map[string]string{"students": "done", "work_time": "skipped"} {
		if _, err := rt.Client.Put("/api/staff-onboarding/steps/"+step, map[string]any{"state": state}); err != nil {
			return fmt.Errorf("set staff first step %s: %w", step, err)
		}
	}

	// seedInactiveAccountStep has offboarded the last guest; it cannot log in.
	offboarded := rt.FixedSeeder.staffCredentials[len(rt.FixedSeeder.staffCredentials)-1].Email
	hidden := 0
	for _, dismissed := range staff[1:] {
		if dismissed.Email == offboarded {
			continue
		}
		hidden++
		if err := rt.Client.Login(dismissed.Email, dismissed.Password); err != nil {
			return fmt.Errorf("login as staff %s: %w", dismissed.Email, err)
		}
		if _, err := rt.Client.Put("/api/staff-onboarding/dismissal", map[string]any{"dismissed": true}); err != nil {
			return fmt.Errorf("dismiss staff first steps for %s: %w", dismissed.Email, err)
		}
	}

	fmt.Printf("  staff first steps: one started, %d hidden\n", hidden)
	return nil
}
