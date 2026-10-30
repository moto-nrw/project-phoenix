package api

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalCarePlanAccountIsSavedByFullWorkflow(t *testing.T) {
	t.Parallel()
	srv := fullSeedAPIMock(t)
	defer srv.Close()
	statePath := filepath.Join(t.TempDir(), DefaultSeedStatePath)
	s := NewSeeder(newSeedTestAdapter(srv.URL), newSeedTestRandom(), false, SeedOptions{
		StatePath: statePath, OnlyProfile: DefaultProfileKey,
		StaffPassword: "example-test-password",
	})
	_, err := s.Seed(t.Context(), "operator@example.test", "example-test-password", "1234")
	require.NoError(t, err)
	state, err := LoadSeedState(statePath)
	require.NoError(t, err)
	profile, err := state.SelectProfile(DefaultProfileKey)
	require.NoError(t, err)
	require.Len(t, profile.Credentials.Accounts.Local, 1)
	account := profile.Credentials.Accounts.Local[0]
	assert.Equal(t, localCarePlanAccountKey, account.Key)
	assert.Equal(t, "example-test-password", account.Password)
	assert.Positive(t, account.AccountID)
	assert.Positive(t, account.StaffID)
}

func TestLocalCarePlanAccountIsLocalOnly(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		url     string
		options SeedOptions
		want    bool
	}{
		{"localhost", "http://localhost:8080", SeedOptions{}, true},
		{"IPv4", "http://127.0.0.1:8080", SeedOptions{}, true},
		{"IPv6", "http://[::1]:8080", SeedOptions{}, true},
		{"remote", "https://example.test", SeedOptions{}, false},
		{"standing demo on loopback", "http://localhost:8080", SeedOptions{StandingDemo: true}, false},
		{"visitor demo on loopback", "http://localhost:8080", SeedOptions{VisitorFirstName: "Test"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := NewSeeder(newSeedTestAdapter(tc.url), newSeedTestRandom(), false, tc.options)
			step := seedLocalCarePlanAccountStep{seeder: s}
			assert.Equal(t, tc.want, step.enabled())
			if !tc.want {
				require.NoError(t, step.Run(t.Context(), &Runtime{}), "excluded seeds must make no API calls")
			}
		})
	}
}

func TestLocalCarePlanAccountHasOnlyItsReadPlanningRole(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"", "ogs-nord", "lokaler-testmandant-5300"} {
		t.Run("scope="+scope, func(t *testing.T) {
			t.Parallel()
			assertLocalCarePlanAccountRole(t, scope)
		})
	}
}

func assertLocalCarePlanAccountRole(t *testing.T, scope string) {
	t.Helper()
	var registration map[string]any
	var granted []string
	createdRoles := 0
	catalog := seedTestPermissionCatalog()
	permissionNames := map[string]string{}
	for _, permission := range catalog {
		permissionNames[permission["id"].(string)] = permission["name"].(string)
	}
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /auth/permissions":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": catalog})
		case "POST /auth/roles":
			var role map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&role))
			assert.Equal(t, localCarePlanAccountKey, role["name"])
			assert.Equal(t, "user", role["base_role"])
			createdRoles++
			_, _ = fmt.Fprint(w, `{"data":{"id":"7300"}}`)
		case "PUT /auth/roles/7300/permissions":
			var body struct {
				IDs []string `json:"permission_ids"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			for _, id := range body.IDs {
				granted = append(granted, permissionNames[id])
			}
			_, _ = fmt.Fprint(w, `{"data":{}}`)
		case "POST /auth/register":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&registration))
			_, _ = fmt.Fprint(w, `{"data":{"id":8300,"school_identity":{"staff_id":"9300"}}}`)
		default:
			t.Errorf("unexpected request %s %s: no extra role or direct grants allowed", r.Method, r.URL.Path)
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()
	s := NewSeeder(newSeedTestAdapter(srv.URL), newSeedTestRandom(), false, SeedOptions{})
	fs := NewFixedSeeder(s.client, false, "")
	fs.accountScope = scope
	fs.staffCredentials = []StaffCredentials{{Password: "example-test-password"}}
	rt := &Runtime{Client: s.client, FixedSeeder: fs}
	require.NoError(t, (seedLocalCarePlanAccountStep{seeder: s}).Run(t.Context(), rt))
	assert.Equal(t, 1, createdRoles)
	assert.EqualValues(t, 7300, registration["role_id"])
	assert.NotContains(t, registration, "role_ids")
	assert.Equal(t, scopedEmail("nachbearbeitung-ohne-planung@example.test", scope), registration["email"])
	username := registration["username"].(string)
	assert.LessOrEqual(t, len(username), 30, "registration must accept scoped local accounts")
	if scope != "" {
		assert.Equal(t, "np-"+scope, username)
	}
	assert.Equal(t, fs.staffCredentials[0].Password, registration["password"])
	assert.Equal(t, registration["password"], registration["confirm_password"])
	for _, permission := range []string{"config:manage", "config:read", "users:read", "schedules:list", "schedules:read"} {
		assert.Contains(t, granted, permission)
	}
	for _, permission := range granted {
		assert.NotContains(t, permission, "*")
		if strings.HasPrefix(permission, "schedules:") {
			assert.Contains(t, []string{"schedules:list", "schedules:read"}, permission)
		}
	}
	state := &SeedState{}
	s.populateSeedAccounts(state, fs)
	require.Len(t, state.Accounts.Local, 1)
	assert.Equal(t, localCarePlanAccountKey, state.Accounts.Local[0].Key)
	assert.EqualValues(t, 8300, state.Accounts.Local[0].AccountID)
	assert.EqualValues(t, 9300, state.Accounts.Local[0].StaffID)
	assert.Empty(t, state.Accounts.Admin)
	assert.Empty(t, state.Accounts.Betreuer)
	definitions := demoRoleDefinitions()
	require.Len(t, definitions, 2, "public demo roles stay separate")
	assert.Equal(t, demoRoleCaregiverName, definitions[0].Name)
	assert.Equal(t, demoRoleLeadName, definitions[1].Name)
	assert.NotContains(t, definitions[0].Permissions, "config:manage")
	assert.Contains(t, definitions[1].Permissions, "schedules:manage")
}
