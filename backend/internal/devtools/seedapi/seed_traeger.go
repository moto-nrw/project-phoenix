package api

import (
	"context"
	"fmt"
)

// seedTraegerBueroStep makes the bootstrap school admin the first person of
// the Träger-Büro (ADR 0084, #4649) through the operator route, so every dev
// and demo database can sign in to the Träger-Portal with the admin's
// credentials and sees the Träger's schools.
type seedTraegerBueroStep struct{}

func (seedTraegerBueroStep) Name() string { return "Seeding the Träger-Büro" }

func (seedTraegerBueroStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.OrganizationID <= 0 || rt.Bootstrap.AdminEmail == "" {
		return fmt.Errorf("the Träger-Büro seed prerequisites are not available")
	}
	previousAuth := rt.Client.auth
	rt.Client.BindAuth(rt.OperatorAuth)
	defer rt.Client.BindAuth(previousAuth)
	path := fmt.Sprintf("/operator/organizations/%d/traeger-members", rt.Bootstrap.OrganizationID)
	if _, err := rt.Client.Post(path, map[string]string{"email": rt.Bootstrap.AdminEmail, "role": "administration"}); err != nil {
		return fmt.Errorf("assign the Träger-Büro: %w", err)
	}
	fmt.Println("  Träger-Büro: school admin assigned with role Verwaltung")
	return nil
}
