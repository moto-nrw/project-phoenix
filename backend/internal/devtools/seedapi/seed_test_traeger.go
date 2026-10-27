package api

import (
	"context"
	"fmt"
)

// seedTestTraegerStep creates a test Träger with test schools and invented
// sample children (#4658) through the operator routes, so every dev and demo
// database shows how the moto team sets up a test system. A second seed run
// finds the Träger already there and leaves it alone.
type seedTestTraegerStep struct{}

const seedTestTraegerSlug = "test-traeger"

func (seedTestTraegerStep) Name() string { return "Seeding a test Träger" }

func (seedTestTraegerStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" {
		return fmt.Errorf("the test Träger seed prerequisites are not available")
	}
	previousAuth := rt.Client.auth
	rt.Client.BindAuth(rt.OperatorAuth)
	defer rt.Client.BindAuth(previousAuth)
	raw, err := rt.Client.Post("/operator/organizations/test-traeger", map[string]any{
		"name": "Test-Träger", "slug": seedTestTraegerSlug, "school_count": 3, "contact_email": rt.Bootstrap.AdminEmail,
	})
	if isConflictError(err) {
		fmt.Println("  Test-Träger: already present")
		return nil
	}
	if err != nil {
		return fmt.Errorf("create the test Träger: %w", err)
	}
	var created struct {
		Data struct {
			Organization struct {
				ID int64 `json:"id"`
			} `json:"organization"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &created); err != nil || created.Data.Organization.ID <= 0 {
		return fmt.Errorf("decode the test Träger: %w", err)
	}
	path := fmt.Sprintf("/operator/organizations/%d/sample-children", created.Data.Organization.ID)
	if _, err := rt.Client.Post(path, map[string]any{}); err != nil {
		return fmt.Errorf("fill the test Träger with sample children: %w", err)
	}
	fmt.Println("  Test-Träger: three test schools with sample children")
	return nil
}
