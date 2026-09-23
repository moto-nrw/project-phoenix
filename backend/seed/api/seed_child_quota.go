package api

import (
	"context"
	"fmt"
)

// childQuotaSeedHeadroom is how many places the demo school keeps free, so
// the Kinderkontingent is almost full on every dev machine (#3567): a few
// more children fit, then creating one is refused.
const childQuotaSeedHeadroom = 3

// seedChildQuotaStep gives the full demo school a Kinderkontingent just above
// its current children through the operator school route. It runs after
// every step that adds children, so none of them is refused.
type seedChildQuotaStep struct{}

func (seedChildQuotaStep) Name() string { return "Seeding the Kinderkontingent" }

func (seedChildQuotaStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil {
		return fmt.Errorf("child quota demo prerequisites not available")
	}
	children, err := runningChildren(rt)
	if err != nil {
		return err
	}
	school, err := findOperatorSchool(rt, rt.Bootstrap.SchoolID)
	if err != nil {
		return err
	}
	// One bundle whose size is the demo's contract: a special size is exactly
	// what the operator uses for contracts off the standard 50.
	bundleSize := children + childQuotaSeedHeadroom
	school["child_quota"] = map[string]any{"bundles": 1, "bundle_size": bundleSize}
	if _, err := rt.Client.PutWithAuth(rt.OperatorAuth, fmt.Sprintf("/operator/schools/%d", rt.Bootstrap.SchoolID), school); err != nil {
		return fmt.Errorf("set child quota: %w", err)
	}
	fmt.Printf("  Kinderkontingent: %d places, %d children in care\n", bundleSize, children)
	return nil
}

// runningChildren counts the demo school's children in running care.
func runningChildren(rt *Runtime) (int, error) {
	raw, err := rt.Client.GetWithAuth(rt.TenantAuth, "/api/students?page_size=1")
	if err != nil {
		return 0, fmt.Errorf("count children: %w", err)
	}
	var payload struct {
		Pagination struct {
			TotalRecords int `json:"total_records"`
		} `json:"pagination"`
	}
	if err := parseJSON(raw, &payload); err != nil {
		return 0, fmt.Errorf("parse children count: %w", err)
	}
	return payload.Pagination.TotalRecords, nil
}

// findOperatorSchool reads the school's editable fields, because the
// operator school update replaces all of them.
func findOperatorSchool(rt *Runtime, schoolID int64) (map[string]any, error) {
	raw, err := rt.Client.GetWithAuth(rt.OperatorAuth, "/operator/schools")
	if err != nil {
		return nil, fmt.Errorf("list schools: %w", err)
	}
	var payload struct {
		Data []struct {
			ID             int64  `json:"id"`
			OrganizationID int64  `json:"organization_id"`
			Name           string `json:"name"`
			Slug           string `json:"slug"`
			Subdomain      string `json:"subdomain"`
			Address        string `json:"address"`
			City           string `json:"city"`
			Zip            string `json:"zip"`
			Phone          string `json:"phone"`
			Email          string `json:"email"`
			Active         bool   `json:"active"`
			Hidden         bool   `json:"hidden"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse schools response: %w", err)
	}
	for _, school := range payload.Data {
		if school.ID != schoolID {
			continue
		}
		return map[string]any{
			"organization_id": school.OrganizationID, "name": school.Name, "slug": school.Slug,
			"subdomain": school.Subdomain, "address": school.Address, "city": school.City, "zip": school.Zip,
			"phone": school.Phone, "email": school.Email, "active": school.Active, "hidden": school.Hidden,
		}, nil
	}
	return nil, fmt.Errorf("school %d not found", schoolID)
}
