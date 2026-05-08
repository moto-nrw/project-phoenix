package contract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moto-nrw/project-phoenix/e2e/scenarios"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAndLoadManifest_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e2e-manifest.json")

	original := &Manifest{
		Version: ManifestVersion,
		Runtime: Runtime{
			BackendURL:       "http://localhost:8081",
			TenantDomain:     "localtest.me",
			FrontendPort:     3030,
			OperatorHostname: "operator.localtest.me:3030",
		},
		Scenario: ScenarioMetadata{Name: scenarios.NameE2EMultiTenant, Mode: scenarios.ModeMultiTenant},
		Setup: Setup{
			Auth: AuthSetup{
				Roles:                     []string{scenarios.SetupRoleAdmin, scenarios.SetupRoleStaff},
				RequiresSecondaryTenant:   true,
				RequiresVerifiedSwitching: true,
			},
		},
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
		Switching: &Switching{
			LinkEmail: "demo1@mail.de",
			Verified:  true,
			Actor: Actor{
				Email:       "demo1@mail.de",
				Password:    "pass1",
				DisplayName: "Anna Mueller",
				Role:        "admin",
				StaffID:     10,
			},
		},
		Devices: Devices{
			DefaultCheckin: Device{
				Key:    "device-001",
				APIKey: "key-abc",
				PIN:    "1234",
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

	err := WriteManifest(original, path)
	require.NoError(t, err)

	loaded, err := LoadManifest(path)
	require.NoError(t, err)

	assert.Equal(t, ManifestVersion, loaded.Version)
	assert.Equal(t, "http://localhost:8081", loaded.Runtime.BackendURL)
	assert.Equal(t, "localtest.me", loaded.Runtime.TenantDomain)
	assert.Equal(t, 3030, loaded.Runtime.FrontendPort)
	assert.Equal(t, "operator.localtest.me:3030", loaded.Runtime.OperatorHostname)
	assert.Equal(t, scenarios.NameE2EMultiTenant, loaded.Scenario.Name)
	assert.Equal(t, []string{scenarios.SetupRoleAdmin, scenarios.SetupRoleStaff}, loaded.Setup.Auth.Roles)
	assert.True(t, loaded.Setup.Auth.RequiresSecondaryTenant)
	assert.True(t, loaded.Setup.Auth.RequiresVerifiedSwitching)
	assert.Equal(t, "demo-school", loaded.Tenants.Primary.Slug)
	require.NotNil(t, loaded.Tenants.Secondary)
	assert.Equal(t, "second-school", loaded.Tenants.Secondary.Slug)
	assert.Equal(t, "demo1@mail.de", loaded.Actors.Admin.Email)
	require.NotNil(t, loaded.Switching)
	assert.True(t, loaded.Switching.Verified)
	assert.Equal(t, "demo1@mail.de", loaded.Switching.Actor.Email)
	assert.Equal(t, "device-001", loaded.Devices.DefaultCheckin.Key)
	assert.Equal(t, "key-abc", loaded.Devices.DefaultCheckin.APIKey)
	assert.Equal(t, "1234", loaded.Devices.DefaultCheckin.PIN)
	assert.Equal(t, "E2EFE110001", loaded.Fixtures.Checkin.RFIDTag)
	assert.Equal(t, "Felix", loaded.Fixtures.Checkin.Student.FirstName)
	assert.Equal(t, "Hausaufgaben", loaded.Fixtures.Checkin.Activity.Name)
	assert.Equal(t, "Bärengruppe", loaded.Fixtures.Groups.VisiblePair.Primary.DisplayName)
}

func TestWriteManifest_CreatesFileWithRestrictedPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e2e-manifest.json")

	err := WriteManifest(&Manifest{}, path)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestManifestNormalize_FillsRuntimeDefaults(t *testing.T) {
	manifest := &Manifest{}

	manifest.Normalize()

	assert.Equal(t, ManifestVersion, manifest.Version)
	assert.Equal(t, DefaultBackendURL, manifest.Runtime.BackendURL)
	assert.Equal(t, DefaultTenantDomain, manifest.Runtime.TenantDomain)
	assert.Equal(t, DefaultFrontendPort, manifest.Runtime.FrontendPort)
	assert.Equal(t, "operator.localtest.me:3030", manifest.Runtime.OperatorHostname)
	assert.Equal(t, scenarios.ModeSingleTenant, manifest.Scenario.Mode)
	assert.Equal(t, []string{scenarios.SetupRoleAdmin, scenarios.SetupRoleStaff}, manifest.Setup.Auth.Roles)
}
