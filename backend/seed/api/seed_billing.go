package api

import (
	"context"
	"fmt"
)

// seedBillingKeyDateCountsStep gives every fresh demo a synthetic current-day
// billing snapshot through the local-only operator route. It keeps the report
// reviewable without changing the worker-only production capture path.
type seedBillingKeyDateCountsStep struct{}

func (seedBillingKeyDateCountsStep) Name() string { return "Seeding billing key-date counts" }

func (seedBillingKeyDateCountsStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil {
		return fmt.Errorf("billing demo prerequisites not available")
	}
	previousAuth := rt.Client.auth
	rt.Client.BindAuth(rt.OperatorAuth)
	defer rt.Client.BindAuth(previousAuth)
	if _, err := rt.Client.PostWithHeaders("/operator/billing/key-date-counts/seed", nil, map[string]string{
		seedTokenHeader: "true",
	}); err != nil {
		return fmt.Errorf("seed billing key-date counts: %w", err)
	}
	fmt.Println("  billing key-date counts captured")
	return nil
}
