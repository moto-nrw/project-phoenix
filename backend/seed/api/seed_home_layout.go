package api

import (
	"context"
	"fmt"
)

// seedHomeLayoutStep gives the demo school's start page an individual choice
// and a school-wide prescription. Both writes go through their public API so
// the seeded stack demonstrates the precedence rule as well as covering the
// two settings-platform tables.
type seedHomeLayoutStep struct{}

func (seedHomeLayoutStep) Name() string { return "Seeding start page layout" }

func (seedHomeLayoutStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil {
		return fmt.Errorf("start page layout seed prerequisites not available")
	}
	rt.Client.BindAuth(rt.TenantAuth)

	if _, err := rt.Client.Put("/api/settings/home-layout", map[string]any{
		"overrides": map[string]bool{
			"section.birthdays": false,
		},
	}); err != nil {
		return fmt.Errorf("seed personal start page layout: %w", err)
	}
	if _, err := rt.Client.Put("/api/settings/home-layout/policies", map[string]any{
		"policies": map[string]string{
			"tile.students_sick": "required",
			"tile.students_home": "disabled",
		},
	}); err != nil {
		return fmt.Errorf("seed school start page layout: %w", err)
	}
	fmt.Println("  1 personal start page choice and 2 school-wide prescriptions created")
	return nil
}
