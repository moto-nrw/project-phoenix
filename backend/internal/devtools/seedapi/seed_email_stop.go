package api

import (
	"context"
	"fmt"
	"slices"
)

// seedEmailStopStep stops mail to one guardian through the route a school uses
// (#5934), so the guardian tab shows „Versand gestoppt“. It runs after every
// invitation of the seed, because a stopped person gets no new invitation.
type seedEmailStopStep struct{}

func (seedEmailStopStep) Name() string { return "Seeding a stopped guardian address" }

func (seedEmailStopStep) Run(ctx context.Context, rt *Runtime) error {
	if rt == nil || rt.Adapter == nil || rt.Client == nil || rt.Bootstrap == nil || rt.FixedSeeder == nil || len(rt.FixedSeeder.guardianIDs) == 0 {
		return fmt.Errorf("guardian email stop seed prerequisites not available")
	}
	ids := make([]int64, 0, len(rt.FixedSeeder.guardianIDs))
	for _, id := range rt.FixedSeeder.guardianIDs {
		ids = append(ids, id)
	}
	auth, err := rt.Adapter.LoginTenant(ctx, rt.Bootstrap.AdminEmail, rt.Bootstrap.AdminPassword, rt.Bootstrap.TenantSlug)
	if err != nil {
		return fmt.Errorf("login admin for the guardian email stop: %w", err)
	}
	guardianID := slices.Max(ids)
	if _, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/api/guardians/%d/email-stop", guardianID), map[string]any{}); err != nil {
		return fmt.Errorf("seed the guardian email stop: %w", err)
	}
	fmt.Printf("  mail to guardian %d stopped\n", guardianID)
	return nil
}
