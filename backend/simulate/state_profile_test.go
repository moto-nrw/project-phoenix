package simulate

import (
	"path/filepath"
	"testing"

	"github.com/moto-nrw/project-phoenix/demoprofile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSeedStateProfile_SelectsDefaultAndExplicitProfile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "state.json")
	contract := &demoprofile.SeedState{
		Version: demoprofile.CurrentSeedStateVersion, DefaultProfile: demoprofile.DefaultProfileKey,
		Profiles: map[string]*demoprofile.SeedProfile{
			demoprofile.DefaultProfileKey: simulationTestProfile(demoprofile.DefaultProfileKey, "vollbetrieb", "full@example.test"),
			"klein":                       simulationTestProfile("klein", "klein", "klein@example.test"),
		},
	}
	require.NoError(t, demoprofile.WriteSeedState(contract, path))

	defaultState, err := LoadSeedState(path)
	require.NoError(t, err)
	assert.Equal(t, demoprofile.DefaultProfileKey, defaultState.ProfileKey)
	assert.Equal(t, "vollbetrieb", defaultState.Bootstrap.TenantSlug)

	explicitState, err := LoadSeedStateProfile(path, "klein")
	require.NoError(t, err)
	assert.Equal(t, "klein", explicitState.ProfileKey)
	assert.Equal(t, "klein@example.test", explicitState.Accounts.Admin[0].Email)
}

func TestLoadSeedStateProfile_RejectsUnknownProfile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "state.json")
	contract := &demoprofile.SeedState{
		Version: demoprofile.CurrentSeedStateVersion, DefaultProfile: demoprofile.DefaultProfileKey,
		Profiles: map[string]*demoprofile.SeedProfile{
			demoprofile.DefaultProfileKey: simulationTestProfile(demoprofile.DefaultProfileKey, "vollbetrieb", "full@example.test"),
		},
	}
	require.NoError(t, demoprofile.WriteSeedState(contract, path))

	_, err := LoadSeedStateProfile(path, "fehlt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown demo school profile "fehlt"`)
}

func simulationTestProfile(key, slug, email string) *demoprofile.SeedProfile {
	return &demoprofile.SeedProfile{
		Key: key, School: demoprofile.SeedSchoolRef{TenantSlug: slug},
		Credentials: demoprofile.SeedStateCredentials{
			DevicePIN: "1234",
			Accounts:  demoprofile.SeedStateAccounts{Admin: []demoprofile.AccountCredentials{{Email: email, Password: "Test1234%"}}},
		},
		Devices: map[string]demoprofile.SeedDevice{"terminal": {APIKey: "key", Name: "Terminal"}},
		Entities: demoprofile.SeedProfileEntities{
			Students: map[string]demoprofile.SeedStudent{}, Rooms: map[string]demoprofile.SeedEntityRef{},
			Activities: map[string]demoprofile.SeedEntityRef{}, Groups: map[string]demoprofile.SeedEntityRef{},
		},
	}
}
