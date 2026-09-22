package api

import (
	"context"
	"fmt"
)

// seedSchoolSetupStep finishes the onboarding wizard (#2832) for the demo
// school. The seeded school is fully set up and must not greet anybody with
// the wizard; a school the operator creates later still does. Every write goes
// through the public wizard routes, so the step also covers both tables:
// config.school_setups through the completion and
// config.school_setup_dismissals through the admin hiding the wizard.
type seedSchoolSetupStep struct{}

func (seedSchoolSetupStep) Name() string { return "Completing school setup" }

type seedSchoolSetupStatus struct {
	Data struct {
		Basics struct {
			PresenceMode string `json:"presence_mode"`
		} `json:"basics"`
		Steps []struct {
			Key     string `json:"key"`
			Applies bool   `json:"applies"`
			Done    bool   `json:"done"`
			Skipped bool   `json:"skipped"`
		} `json:"steps"`
	} `json:"data"`
}

func (seedSchoolSetupStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil {
		return fmt.Errorf("school setup seed prerequisites not available")
	}
	rt.Client.BindAuth(rt.TenantAuth)

	raw, err := rt.Client.Get("/api/school-setup")
	if err != nil {
		return fmt.Errorf("read school setup: %w", err)
	}
	var status seedSchoolSetupStatus
	if err := parseJSON(raw, &status); err != nil {
		return fmt.Errorf("decode school setup: %w", err)
	}
	// Confirm the presence mode the profile already configured: the wizard
	// must not change how the demo school works.
	if _, err := rt.Client.Put("/api/school-setup/basics", map[string]any{
		"presence_mode":   status.Data.Basics.PresenceMode,
		"parent_app_used": true,
	}); err != nil {
		return fmt.Errorf("confirm school setup basics: %w", err)
	}
	skipped := 0
	for _, step := range status.Data.Steps {
		if step.Key == "basics" || !step.Applies || step.Done || step.Skipped {
			continue
		}
		if _, err := rt.Client.Put("/api/school-setup/steps/"+step.Key, map[string]any{"skipped": true}); err != nil {
			return fmt.Errorf("skip school setup step %s: %w", step.Key, err)
		}
		skipped++
	}
	if _, err := rt.Client.Post("/api/school-setup/complete", nil); err != nil {
		return fmt.Errorf("complete school setup: %w", err)
	}
	if _, err := rt.Client.Put("/api/school-setup/dismissal", map[string]any{"dismissed": true}); err != nil {
		return fmt.Errorf("dismiss school setup: %w", err)
	}
	fmt.Printf("  school setup completed (%d open steps skipped)\n", skipped)
	return nil
}
