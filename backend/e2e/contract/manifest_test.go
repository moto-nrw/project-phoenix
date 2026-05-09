package contract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moto-nrw/project-phoenix/e2e/scenarios"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAndLoadState_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e2e-state.json")

	original := &State{
		Version: StateVersion,
		Runtime: Runtime{
			BackendURL:       "http://localhost:8081",
			TenantDomain:     "localtest.me",
			FrontendPort:     3030,
			OperatorHostname: "operator.localtest.me:3030",
			NextAuthSecret:   "nextauth-secret",
			AuthTrustHost:    true,
		},
		World: World{
			Scenario: ScenarioMetadata{Name: scenarios.NameE2EMultiTenant, Mode: scenarios.ModeMultiTenant},
			Tenants: Tenants{
				Primary: Tenant{
					OrganizationID: 1,
					SchoolID:       2,
					Slug:           "demo-school",
					Name:           "Demo School",
				},
				Secondary: &Tenant{
					OrganizationID: 1,
					SchoolID:       3,
					Slug:           "second-school",
					Name:           "Demo School 2",
				},
			},
			Actors: Actors{
				Admin: Actor{
					Email:       "demo1@mail.de",
					Password:    "pass1",
					DisplayName: "Anna Mueller",
					Role:        "admin",
					StaffID:     10,
				},
				Staff: Actor{
					Email:       "demo11@mail.de",
					Password:    "pass2",
					DisplayName: "Julia Klein",
					Role:        "user",
					StaffID:     20,
				},
			},
			Devices: Devices{
				DefaultCheckin: Device{
					Key:    "device-001",
					APIKey: "key-abc",
					PIN:    "1234",
				},
			},
		},
		Setup: Setup{
			Auth: AuthSetup{
				Roles:                     []string{scenarios.SetupRoleAdmin, scenarios.SetupRoleStaff},
				RequiresSecondaryTenant:   true,
				RequiresVerifiedSwitching: true,
			},
		},
		Assertions: Assertions{
			Switching: SwitchingAssertion{
				Required:  true,
				Verified:  true,
				LinkEmail: "demo1@mail.de",
				Actor: &Actor{
					Email:       "demo1@mail.de",
					Password:    "pass1",
					DisplayName: "Anna Mueller",
					Role:        "admin",
					StaffID:     10,
				},
			},
		},
		Fixtures: Fixtures{
			Students: StudentFixtures{
				SearchPair: StudentPair{
					Primary: StudentRef{
						ID:        100,
						FirstName: "Felix",
						LastName:  "Schneider",
						GroupKey:  "sternengruppe",
						Class:     "Klasse 1a",
					},
					Secondary: StudentRef{
						ID:        101,
						FirstName: "Emma",
						LastName:  "Meyer",
						GroupKey:  "sternengruppe",
						Class:     "Klasse 1a",
					},
				},
				SickPresent: StudentRef{
					ID:        102,
					FirstName: "Leon",
					LastName:  "Koch",
					GroupKey:  "sternengruppe",
					Class:     "Klasse 1a",
				},
			},
			Groups: GroupFixtures{
				VisiblePair: GroupPair{
					Primary:   GroupRef{ID: 50, Key: "bärengruppe", DisplayName: "Bärengruppe"},
					Secondary: GroupRef{ID: 51, Key: "sternengruppe", DisplayName: "Sternengruppe"},
				},
			},
			Checkin: CheckinFixture{
				Student: StudentRef{
					ID:        100,
					FirstName: "Felix",
					LastName:  "Schneider",
					GroupKey:  "sternengruppe",
					Class:     "Klasse 1a",
				},
				Room:       RoomRef{ID: 10, Name: "OGS-Raum 1"},
				Activity:   ActivityRef{ID: 50, Name: "Hausaufgaben"},
				DeviceKey:  "device-001",
				RFIDTag:    "E2EFE110001",
				Supervisor: Actor{Email: "demo1@mail.de", Password: "pass1", DisplayName: "Anna Mueller", Role: "admin", StaffID: 10},
			},
		},
	}

	err := WriteState(original, path)
	require.NoError(t, err)

	loaded, err := LoadState(path)
	require.NoError(t, err)

	assert.Equal(t, StateVersion, loaded.Version)
	assert.Equal(t, "http://localhost:8081", loaded.Runtime.BackendURL)
	assert.Equal(t, "localtest.me", loaded.Runtime.TenantDomain)
	assert.Equal(t, 3030, loaded.Runtime.FrontendPort)
	assert.Equal(t, scenarios.NameE2EMultiTenant, loaded.World.Scenario.Name)
	assert.Equal(t, "demo-school", loaded.World.Tenants.Primary.Slug)
	require.NotNil(t, loaded.World.Tenants.Secondary)
	assert.Equal(t, "second-school", loaded.World.Tenants.Secondary.Slug)
	assert.Equal(t, "demo1@mail.de", loaded.World.Actors.Admin.Email)
	assert.True(t, loaded.Assertions.Switching.Verified)
	require.NotNil(t, loaded.Assertions.Switching.Actor)
	assert.Equal(t, "demo1@mail.de", loaded.Assertions.Switching.Actor.Email)
	assert.Equal(t, "E2EFE110001", loaded.Fixtures.Checkin.RFIDTag)
}

func TestWriteState_CreatesHostReadableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e2e-state.json")

	err := WriteState(validE2EState(), path)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func TestStateNormalize_SetsVersionOnly(t *testing.T) {
	state := &State{}

	state.Normalize()

	assert.Equal(t, StateVersion, state.Version)
	assert.Empty(t, state.Runtime.BackendURL)
	assert.Empty(t, state.World.Scenario.Mode)
	assert.Empty(t, state.Setup.Auth.Roles)
}

func TestWriteState_RejectsIncompleteRuntime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e2e-state.json")

	err := WriteState(&State{}, path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "state.runtime.backend_url is required")
}

func TestWriteState_RejectsIncompleteFixtureFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e2e-state.json")
	state := validE2EState()
	state.Fixtures.Checkin.Room.ID = 0

	err := WriteState(state, path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "state.fixtures.checkin.room.id must be > 0")
}

func TestWriteState_RejectsVerifiedSwitchingWithoutActor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e2e-state.json")
	state := validE2EState()
	state.Assertions.Switching.Actor = nil

	err := WriteState(state, path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "state.assertions.switching.actor is required")
}

func validE2EState() *State {
	return &State{
		Version: StateVersion,
		Runtime: Runtime{
			BackendURL:       "http://localhost:8081",
			TenantDomain:     "localtest.me",
			FrontendPort:     3030,
			OperatorHostname: "operator.localtest.me:3030",
			NextAuthSecret:   "nextauth-secret",
			AuthTrustHost:    true,
		},
		World: World{
			Scenario: ScenarioMetadata{Name: scenarios.NameE2EMultiTenant, Mode: scenarios.ModeMultiTenant},
			Tenants: Tenants{
				Primary: Tenant{
					OrganizationID: 1,
					SchoolID:       2,
					Slug:           "demo-school",
					Name:           "Demo School",
				},
				Secondary: &Tenant{
					OrganizationID: 1,
					SchoolID:       3,
					Slug:           "second-school",
					Name:           "Demo School 2",
				},
			},
			Actors: Actors{
				Admin: Actor{
					Email:       "demo1@mail.de",
					Password:    "pass1",
					DisplayName: "Anna Mueller",
					Role:        "admin",
					StaffID:     10,
				},
				Staff: Actor{
					Email:       "demo11@mail.de",
					Password:    "pass2",
					DisplayName: "Julia Klein",
					Role:        "user",
					StaffID:     20,
				},
				Operator: &OperatorCredentials{
					Email:    "operator@e2e.local",
					Password: "operator-pass",
				},
			},
			Devices: Devices{
				DefaultCheckin: Device{
					Key:    "device-001",
					APIKey: "key-abc",
					PIN:    "1234",
				},
			},
		},
		Setup: Setup{
			Auth: AuthSetup{
				Roles:                     []string{scenarios.SetupRoleAdmin, scenarios.SetupRoleStaff},
				RequiresSecondaryTenant:   true,
				RequiresVerifiedSwitching: true,
			},
		},
		Assertions: Assertions{
			Switching: SwitchingAssertion{
				Required:  true,
				Verified:  true,
				LinkEmail: "demo1@mail.de",
				Actor: &Actor{
					Email:       "demo1@mail.de",
					Password:    "pass1",
					DisplayName: "Anna Mueller",
					Role:        "admin",
					StaffID:     10,
				},
			},
		},
		Fixtures: Fixtures{
			Students: StudentFixtures{
				SearchPair: StudentPair{
					Primary: StudentRef{
						ID:        100,
						FirstName: "Felix",
						LastName:  "Schneider",
						GroupKey:  "sternengruppe",
						Class:     "Klasse 1a",
					},
					Secondary: StudentRef{
						ID:        101,
						FirstName: "Emma",
						LastName:  "Meyer",
						GroupKey:  "sternengruppe",
						Class:     "Klasse 1a",
					},
				},
				SickPresent: StudentRef{
					ID:        102,
					FirstName: "Leon",
					LastName:  "Koch",
					GroupKey:  "sternengruppe",
					Class:     "Klasse 1a",
				},
			},
			Groups: GroupFixtures{
				VisiblePair: GroupPair{
					Primary:   GroupRef{ID: 50, Key: "bärengruppe", DisplayName: "Bärengruppe"},
					Secondary: GroupRef{ID: 51, Key: "sternengruppe", DisplayName: "Sternengruppe"},
				},
			},
			Checkin: CheckinFixture{
				Student: StudentRef{
					ID:        100,
					FirstName: "Felix",
					LastName:  "Schneider",
					GroupKey:  "sternengruppe",
					Class:     "Klasse 1a",
				},
				Room:       RoomRef{ID: 10, Name: "OGS-Raum 1"},
				Activity:   ActivityRef{ID: 50, Name: "Hausaufgaben"},
				DeviceKey:  "device-001",
				RFIDTag:    "E2EFE110001",
				Supervisor: Actor{Email: "demo1@mail.de", Password: "pass1", DisplayName: "Anna Mueller", Role: "admin", StaffID: 10},
			},
		},
	}
}
