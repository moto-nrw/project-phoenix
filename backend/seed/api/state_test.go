package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAndLoadSeedState_Roundtrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test-state.json")

	original := &SeedState{
		CreatedAt: time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
		BaseURL:   "http://localhost:8080",
		DevicePIN: "1234",
		Bootstrap: SeedStateBootstrap{
			OrganizationID:   1,
			OrganizationName: "Stadt Koeln",
			OrganizationSlug: "stadt-koeln",
			SchoolID:         2,
			SchoolName:       "GGS Europaschule",
			SchoolSlug:       "ggs-europaschule",
			TenantSlug:       "ggs-europaschule",
			SchoolAdmin: BootstrapAdminCredentials{
				Email:    "school-admin@example.com",
				Password: "Test1234%",
				Name:     "Seed Admin",
				Position: "OGS-Buero",
			},
		},
		Accounts: SeedStateAccounts{
			Admin: []AccountCredentials{
				{Email: "admin@test.de", Password: "pass1", PIN: "0001", Name: "Admin User", StaffID: 10},
			},
			Betreuer: []AccountCredentials{
				{Email: "betreuer@test.de", Password: "pass2", PIN: "0002", Name: "Betreuer User", StaffID: 20, TeacherID: 30, Group: "sternengruppe"},
			},
		},
		Devices: map[string]SeedDevice{
			"device-001": {APIKey: "key-abc", Name: "Scanner 1"},
		},
		Students: []SeedStudent{
			{ID: 100, FirstName: "Felix", LastName: "Schneider", GroupKey: "sternengruppe", Class: "Klasse 1a"},
		},
		Rooms:      map[string]int64{"OGS-Raum 1": 10, "Sporthalle": 20},
		Activities: map[string]int64{"Fußball": 50, "Basteln": 60},
		Groups:     map[string]int64{"sternengruppe": 50},
	}

	err := WriteSeedState(original, path)
	require.NoError(t, err)

	loaded, err := LoadSeedState(path)
	require.NoError(t, err)

	assert.Equal(t, original.BaseURL, loaded.BaseURL)
	assert.Equal(t, CurrentSeedStateVersion, loaded.Version)
	assert.Equal(t, original.DevicePIN, loaded.DevicePIN)
	assert.Equal(t, original.CreatedAt, loaded.CreatedAt)
	assert.Equal(t, original.Bootstrap.OrganizationSlug, loaded.Bootstrap.OrganizationSlug)
	assert.Equal(t, original.Bootstrap.TenantSlug, loaded.Bootstrap.TenantSlug)
	assert.Equal(t, original.Bootstrap.SchoolAdmin.Email, loaded.Bootstrap.SchoolAdmin.Email)
	assert.Len(t, loaded.Accounts.Admin, 1)
	assert.Equal(t, "admin@test.de", loaded.Accounts.Admin[0].Email)
	assert.Len(t, loaded.Accounts.Betreuer, 1)
	assert.Equal(t, "sternengruppe", loaded.Accounts.Betreuer[0].Group)
	assert.Equal(t, int64(30), loaded.Accounts.Betreuer[0].TeacherID)
	assert.Len(t, loaded.Devices, 1)
	assert.Equal(t, "key-abc", loaded.Devices["device-001"].APIKey)
	assert.Len(t, loaded.Students, 1)
	assert.Equal(t, int64(100), loaded.Students[0].ID)
	assert.Equal(t, int64(10), loaded.Rooms["OGS-Raum 1"])
	assert.Equal(t, int64(50), loaded.Activities["Fußball"])
	assert.Equal(t, int64(50), loaded.Groups["sternengruppe"])
}

func TestWriteSeedState_CreatesFileWithRestrictedPermissions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	state := &SeedState{BaseURL: "http://localhost:8080"}
	err := WriteSeedState(state, path)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestWriteSeedState_ErrorOnInvalidPath(t *testing.T) {
	t.Parallel()

	state := &SeedState{BaseURL: "http://localhost:8080"}
	err := WriteSeedState(state, "/nonexistent/dir/state.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write seed state")
}

func TestLoadSeedState_ErrorOnMissingFile(t *testing.T) {
	t.Parallel()

	_, err := LoadSeedState("/nonexistent/state.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read seed state")
}

func TestLoadSeedState_ErrorOnInvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))

	_, err := LoadSeedState(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal seed state")
}

func TestLoadSeedState_EmptyState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o600))

	state, err := LoadSeedState(path)
	require.NoError(t, err)
	assert.Empty(t, state.BaseURL)
	assert.Empty(t, state.Students)
	assert.NotNil(t, state.Devices)
	assert.Equal(t, CurrentSeedStateVersion, state.Version)
}
