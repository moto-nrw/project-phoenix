package api

import (
	"context"
	"fmt"
)

// seedAnalyticsConsentStep records the usage analytics answer (#4901) of one
// demo caregiver (allowed) and the first demo parent (refused) through the
// consent route, so the profile and settings cards show both stored states.
// The seeded admin stays undecided on purpose: with a PostHog key configured,
// the consent prompt appears after the admin logs in.
type seedAnalyticsConsentStep struct{}

func (seedAnalyticsConsentStep) Name() string { return "Seeding usage analytics consent" }

func (seedAnalyticsConsentStep) Run(ctx context.Context, rt *Runtime) error {
	if rt == nil || rt.Adapter == nil || rt.Client == nil || rt.Bootstrap == nil ||
		rt.FixedSeeder == nil || len(rt.FixedSeeder.staffCredentials) == 0 || len(rt.Parents) == 0 {
		return fmt.Errorf("usage analytics consent seed prerequisites not available")
	}
	staff := rt.FixedSeeder.staffCredentials[0]
	staffAuth, err := rt.Adapter.LoginTenant(ctx, staff.Email, staff.Password, rt.Bootstrap.TenantSlug)
	if err != nil {
		return fmt.Errorf("login caregiver for analytics consent: %w", err)
	}
	if _, err := rt.Client.PutWithAuth(staffAuth, "/analytics-consent/", map[string]any{"granted": true}); err != nil {
		return fmt.Errorf("seed caregiver analytics consent: %w", err)
	}
	parent := rt.Parents[0]
	parentAuth, err := rt.Adapter.LoginParent(ctx, parent.Email, parent.Password)
	if err != nil {
		return fmt.Errorf("login parent for analytics consent: %w", err)
	}
	if _, err := rt.Client.PutWithAuth(parentAuth, "/analytics-consent/", map[string]any{"granted": false}); err != nil {
		return fmt.Errorf("seed parent analytics refusal: %w", err)
	}
	fmt.Println("  1 caregiver consent and 1 parent refusal recorded; the admin stays undecided")
	return nil
}
