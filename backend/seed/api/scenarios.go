package api

import (
	"fmt"

	contract "github.com/moto-nrw/project-phoenix/e2e/contract"
)

const (
	SeedScenarioE2E = "e2e-multi-tenant"

	e2eScenarioOperatorEmail       = "operator@e2e.local"
	e2eScenarioOperatorPassword    = "E2EOp1234!"
	e2eScenarioOperatorDisplayName = "E2E Operator"
	e2eScenarioStaffPIN            = "1234"
	e2eScenarioTenantSlug          = "demo-school"
	e2eScenarioBootstrapAdminEmail = "admin@e2e.local"
	e2eScenarioStaffPassword       = "E2EPass1234!"
	e2eScenarioSecondTenantSlug    = "second-school"
	e2eScenarioSecondTenantName    = "Demo School 2"
	e2eScenarioSecondAdminEmail    = "admin-b@e2e.local"
	e2eScenarioLinkEmail           = "demo1@mail.de"
	e2eScenarioStaffEmail          = "demo11@mail.de"
	e2eScenarioCheckinRoomName     = "OGS-Raum 1"
	e2eScenarioCheckinActivityName = "Hausaufgaben"
	e2eScenarioCheckinDeviceKey    = "demo-device-001"
	e2eScenarioSearchGroupKey      = "sternengruppe"
	e2eScenarioVisibleGroupKeyA    = "sternengruppe"
	e2eScenarioVisibleGroupKeyB    = "bärengruppe"
)

type namedStudentRef struct {
	FirstName string
	LastName  string
}

var (
	e2eScenarioSearchStudentPrimary   = namedStudentRef{FirstName: "Felix", LastName: "Schneider"}
	e2eScenarioSearchStudentSecondary = namedStudentRef{FirstName: "Emma", LastName: "Meyer"}
	e2eScenarioCheckinStudent         = namedStudentRef{FirstName: "Felix", LastName: "Schneider"}
)

type E2EIdentity struct {
	OperatorEmail       string
	OperatorPassword    string
	OperatorDisplayName string
	StaffPIN            string
}

func CanonicalE2EIdentity() E2EIdentity {
	return E2EIdentity{
		OperatorEmail:       e2eScenarioOperatorEmail,
		OperatorPassword:    e2eScenarioOperatorPassword,
		OperatorDisplayName: e2eScenarioOperatorDisplayName,
		StaffPIN:            e2eScenarioStaffPIN,
	}
}

// ApplyScenarioDefaults centralizes scenario-specific seed semantics so
// wrapper scripts only need to say "seed the e2e multi-tenant world" rather than
// re-expressing its tenant/actor topology with a bag of flags.
func ApplyScenarioDefaults(options *SeedOptions) error {
	if options == nil || options.Scenario == "" {
		return nil
	}

	switch options.Scenario {
	case SeedScenarioE2E:
		if options.TenantSlug == "" {
			options.TenantSlug = e2eScenarioTenantSlug
		}
		if options.StatePath == "" {
			options.StatePath = contract.ManifestPath
		}
		options.E2ERuntime.Normalize()
		if options.StaffPassword == "" {
			options.StaffPassword = e2eScenarioStaffPassword
		}
		if options.AdminEmail == "" {
			options.AdminEmail = e2eScenarioBootstrapAdminEmail
		}

		if options.SecondTenant == nil {
			options.SecondTenant = &SecondTenantOptions{}
		}
		if options.SecondTenant.Slug == "" {
			options.SecondTenant.Slug = e2eScenarioSecondTenantSlug
		}
		if options.SecondTenant.Name == "" {
			options.SecondTenant.Name = e2eScenarioSecondTenantName
		}
		if options.SecondTenant.AdminEmail == "" {
			options.SecondTenant.AdminEmail = e2eScenarioSecondAdminEmail
		}
		if options.SecondTenant.AdminPassword == "" {
			options.SecondTenant.AdminPassword = options.StaffPassword
		}
		if options.SecondTenant.LinkEmail == "" {
			options.SecondTenant.LinkEmail = e2eScenarioLinkEmail
		}
		return nil
	default:
		return fmt.Errorf("unknown seed scenario %q", options.Scenario)
	}
}
