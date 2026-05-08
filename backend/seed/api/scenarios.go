package api

import (
	"fmt"
	"strings"

	contract "github.com/moto-nrw/project-phoenix/e2e/contract"
	"github.com/moto-nrw/project-phoenix/e2e/scenarios"
)

const SeedScenarioE2E = scenarios.NameE2EMultiTenant

type E2EIdentity = scenarios.Identity

func IdentityForScenario(name string) (E2EIdentity, error) {
	definition, ok := scenarios.Lookup(name)
	if !ok {
		return E2EIdentity{}, fmt.Errorf("unknown seed scenario %q", name)
	}
	return definition.Identity, nil
}

// ApplyScenarioDefaults centralizes scenario-specific seed semantics so the
// seeder reads one Go-owned scenario registry instead of reconstructing the
// same world through a bag of ad hoc CLI flags and duplicated constants.
func ApplyScenarioDefaults(options *SeedOptions) error {
	if options == nil || options.Scenario == "" {
		return nil
	}

	if err := rejectScenarioOverrides(*options); err != nil {
		return err
	}

	definition, ok := scenarios.Lookup(options.Scenario)
	if !ok {
		return fmt.Errorf("unknown seed scenario %q", options.Scenario)
	}

	if options.TenantSlug == "" {
		options.TenantSlug = definition.Seed.TenantSlug
	}
	if options.StatePath == "" {
		options.StatePath = contract.StatePath
	}
	if options.StaffPassword == "" {
		options.StaffPassword = definition.Seed.StaffPassword
	}
	if options.AdminEmail == "" {
		options.AdminEmail = definition.Seed.BootstrapAdminEmail
	}

	if options.SecondTenant == nil {
		options.SecondTenant = &SecondTenantOptions{}
	}
	if options.SecondTenant.Slug == "" {
		options.SecondTenant.Slug = definition.Seed.SecondTenant.Slug
	}
	if options.SecondTenant.Name == "" {
		options.SecondTenant.Name = definition.Seed.SecondTenant.Name
	}
	if options.SecondTenant.AdminEmail == "" {
		options.SecondTenant.AdminEmail = definition.Seed.SecondTenant.AdminEmail
	}
	if options.SecondTenant.AdminPassword == "" {
		options.SecondTenant.AdminPassword = options.StaffPassword
	}
	if options.SecondTenant.LinkEmail == "" {
		options.SecondTenant.LinkEmail = definition.Seed.SecondTenant.LinkEmail
	}

	return nil
}

func rejectScenarioOverrides(options SeedOptions) error {
	if options.Scenario == "" {
		return nil
	}

	var overridden []string
	if options.TenantSlug != "" {
		overridden = append(overridden, "tenant slug")
	}
	if options.StaffPassword != "" {
		overridden = append(overridden, "staff password")
	}
	if options.AdminEmail != "" {
		overridden = append(overridden, "admin email")
	}
	if options.SecondTenant != nil {
		if options.SecondTenant.Slug != "" {
			overridden = append(overridden, "second-tenant slug")
		}
		if options.SecondTenant.Name != "" {
			overridden = append(overridden, "second-tenant name")
		}
		if options.SecondTenant.AdminEmail != "" {
			overridden = append(overridden, "second-tenant admin email")
		}
		if options.SecondTenant.AdminPassword != "" {
			overridden = append(overridden, "second-tenant admin password")
		}
		if options.SecondTenant.LinkEmail != "" {
			overridden = append(overridden, "second-tenant link email")
		}
	}
	if options.StatePath != "" && options.StatePath != contract.StatePath {
		overridden = append(overridden, "state path")
	}

	if len(overridden) == 0 {
		return nil
	}

	return fmt.Errorf(
		"seed scenario %q owns its deterministic world; do not override %s",
		options.Scenario,
		strings.Join(overridden, ", "),
	)
}
