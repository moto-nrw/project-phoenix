package scenarios

const (
	NameE2EMultiTenant = "e2e-multi-tenant"

	ModeSingleTenant = "single-tenant"
	ModeMultiTenant  = "multi-tenant"

	SetupRoleAdmin = "admin"
	SetupRoleStaff = "staff"
)

type AuthSetup struct {
	Roles                     []string
	RequiresSecondaryTenant   bool
	RequiresVerifiedSwitching bool
}

type Definition struct {
	Name string
	Mode string
	Auth AuthSetup
}

func DefaultPrepareScenario() Definition {
	return MustLookup(NameE2EMultiTenant)
}

func Lookup(name string) (Definition, bool) {
	switch name {
	case NameE2EMultiTenant:
		return Definition{
			Name: NameE2EMultiTenant,
			Mode: ModeMultiTenant,
			Auth: AuthSetup{
				Roles:                     []string{SetupRoleAdmin, SetupRoleStaff},
				RequiresSecondaryTenant:   true,
				RequiresVerifiedSwitching: true,
			},
		}, true
	default:
		return Definition{}, false
	}
}

func MustLookup(name string) Definition {
	definition, ok := Lookup(name)
	if !ok {
		panic("unknown e2e scenario: " + name)
	}
	return definition
}
