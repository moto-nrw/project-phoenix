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

type Identity struct {
	OperatorEmail       string
	OperatorPassword    string
	OperatorDisplayName string
	StaffPIN            string
}

type SecondTenantSeed struct {
	Slug       string
	Name       string
	AdminEmail string
	LinkEmail  string
}

type SeedDefaults struct {
	TenantSlug          string
	BootstrapAdminEmail string
	StaffPassword       string
	SecondTenant        SecondTenantSeed
}

type StudentNameRef struct {
	FirstName string
	LastName  string
}

type PresentReadyStudentSelection struct {
	Student StudentNameRef
	RFIDTag string
}

type StudentFixtureSelection struct {
	SearchPairPrimary   StudentNameRef
	SearchPairSecondary StudentNameRef
	PresentReady        PresentReadyStudentSelection
}

type GroupFixtureSelection struct {
	VisiblePrimaryKey   string
	VisibleSecondaryKey string
}

type CheckinFixtureSelection struct {
	Student      StudentNameRef
	RoomName     string
	ActivityName string
	DeviceKey    string
	RFIDTag      string
}

type FixtureSelection struct {
	StaffEmail string
	Students   StudentFixtureSelection
	Groups     GroupFixtureSelection
	Checkin    CheckinFixtureSelection
}

type Definition struct {
	Name     string
	Mode     string
	Auth     AuthSetup
	Identity Identity
	Seed     SeedDefaults
	Fixtures FixtureSelection
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
			Identity: Identity{
				OperatorEmail:       "operator@e2e.local",
				OperatorPassword:    "E2EOp1234!",
				OperatorDisplayName: "E2E Operator",
				StaffPIN:            "1234",
			},
			Seed: SeedDefaults{
				TenantSlug:          "demo-school",
				BootstrapAdminEmail: "admin@e2e.local",
				StaffPassword:       "E2EPass1234!",
				SecondTenant: SecondTenantSeed{
					Slug:       "second-school",
					Name:       "Demo School 2",
					AdminEmail: "admin-b@e2e.local",
					LinkEmail:  "demo1@mail.de",
				},
			},
			Fixtures: FixtureSelection{
				StaffEmail: "demo11@mail.de",
				Students: StudentFixtureSelection{
					SearchPairPrimary: StudentNameRef{
						FirstName: "Felix",
						LastName:  "Schneider",
					},
					SearchPairSecondary: StudentNameRef{
						FirstName: "Emma",
						LastName:  "Meyer",
					},
					PresentReady: PresentReadyStudentSelection{
						Student: StudentNameRef{
							FirstName: "Leon",
							LastName:  "Koch",
						},
						RFIDTag: "E2E1EC0C001",
					},
				},
				Groups: GroupFixtureSelection{
					VisiblePrimaryKey:   "sternengruppe",
					VisibleSecondaryKey: "bärengruppe",
				},
				Checkin: CheckinFixtureSelection{
					Student: StudentNameRef{
						FirstName: "Felix",
						LastName:  "Schneider",
					},
					RoomName:     "OGS-Raum 1",
					ActivityName: "Hausaufgaben",
					DeviceKey:    "demo-device-001",
					RFIDTag:      "E2EFE110001",
				},
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
