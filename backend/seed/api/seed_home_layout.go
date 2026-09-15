package api

import (
	"context"
	"fmt"
)

// seedHomeLayoutStep gives the demo school's start page an individual choice
// and a school-wide prescription. Both writes go through their public API so
// the seeded stack demonstrates the precedence rule as well as covering the
// two settings-platform tables.
//
// The prescriptions are chosen so they do not distort the role defaults: the
// required block already sits in every default (so nothing extra is appended)
// and the disabled block sits in none (so no default gets a hole). A required
// figure tile did both — it dangled as a lone tile under the caregiver's
// start page and left the leadership's KPI row one tile short.
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
			"section.staff_notices":   "required",
			"section.recent_activity": "disabled",
		},
	}); err != nil {
		return fmt.Errorf("seed school start page layout: %w", err)
	}
	fmt.Println("  1 personal start page choice and 2 school-wide prescriptions created")
	return nil
}
