package api

import (
	"context"
	"fmt"
)

// seedStaffOnboardingStep gives two care workers of the demo school progress
// in their first steps (#3748), through the public routes and each under the
// person's own login: the first has finished one tour and skipped another, so
// the checklist shows all three states; the second has hidden it. Every other
// seeded staff member has not started and sees the full checklist, which is
// what a person invited into a running school sees.
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

	dismissed := staff[1]
	if err := rt.Client.Login(dismissed.Email, dismissed.Password); err != nil {
		return fmt.Errorf("login as staff %s: %w", dismissed.Email, err)
	}
	if _, err := rt.Client.Put("/api/staff-onboarding/dismissal", map[string]any{"dismissed": true}); err != nil {
		return fmt.Errorf("dismiss staff first steps: %w", err)
	}

	fmt.Println("  staff first steps: one started, one hidden")
	return nil
}
