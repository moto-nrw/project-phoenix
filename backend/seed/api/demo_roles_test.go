package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedTestPermissionCatalog is the permission catalog the fake API serves:
// every permission the demo roles name, with ids the tests can recognise.
func seedTestPermissionCatalog() []map[string]any {
	seen := map[string]bool{}
	var catalog []map[string]any
	for _, definition := range demoRoleDefinitions() {
		for _, name := range definition.Permissions {
			if seen[name] {
				continue
			}
			seen[name] = true
			resource, action, _ := strings.Cut(name, ":")
			catalog = append(catalog, map[string]any{
				"id": fmt.Sprintf("%d", 5000+len(catalog)), "name": name, "resource": resource, "action": action,
			})
		}
	}
	return catalog
}

func TestFixedSeeder_SeedDemoRolesCreatesBothRolesWithTheirPermissionSets(t *testing.T) {
	t.Parallel()

	created := map[string]map[string]any{}
	granted := map[string][]string{}
	idsByRole := map[string]string{}
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/permissions":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": seedTestPermissionCatalog()})
		case r.URL.Path == "/auth/roles" && r.Method == seedHTTPMethodPost:
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			name, _ := body["name"].(string)
			created[name] = body
			id := fmt.Sprintf("%d", 700+len(created))
			idsByRole[id] = name
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"id": id}})
		case strings.HasPrefix(r.URL.Path, "/auth/roles/") && strings.HasSuffix(r.URL.Path, "/permissions") && r.Method == seedHTTPMethodPut:
			var body struct {
				PermissionIDs []string `json:"permission_ids"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/auth/roles/"), "/permissions")
			granted[idsByRole[id]] = body.PermissionIDs
			w.WriteHeader(seedHTTPStatusNoContent)
		default:
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"
	fs := NewFixedSeeder(client, false, "")
	require.NoError(t, fs.seedDemoRoles(context.Background()))

	require.Len(t, created, 2)
	assert.Equal(t, "user", created["betreuungskraft"]["base_role"], "a caregiver of the school, like the standard staff role")
	assert.Equal(t, "admin", created["ogs-leitung"]["base_role"], "a lead of the school, like the administrator")
	assert.EqualValues(t, 701, fs.roleIDs["betreuungskraft"])
	assert.EqualValues(t, 702, fs.roleIDs["ogs-leitung"])

	catalog := map[string]string{}
	for _, permission := range seedTestPermissionCatalog() {
		catalog[permission["name"].(string)] = permission["id"].(string)
	}
	for _, definition := range demoRoleDefinitions() {
		want := make([]string, 0, len(definition.Permissions))
		for _, name := range definition.Permissions {
			want = append(want, catalog[name])
		}
		assert.Equal(t, want, granted[definition.Name], "every permission of %s is granted by its catalog id", definition.Name)
	}
}

func TestFixedSeeder_SeedDemoRolesRefusesAPermissionOutsideTheCatalog(t *testing.T) {
	t.Parallel()

	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/auth/permissions" {
			t.Errorf("no role may be created without a complete catalog, got %s %s", r.Method, r.URL.Path)
			w.WriteHeader(seedHTTPStatusNotFound)
			return
		}
		var catalog []map[string]any
		for _, permission := range seedTestPermissionCatalog() {
			if permission["name"] != "supervision:own" {
				catalog = append(catalog, permission)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": catalog})
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"
	fs := NewFixedSeeder(client, false, "")
	err := fs.seedDemoRoles(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"supervision:own"`)
}

// The lead adds planning and administration to the caregiver's day; it does
// not take anything away, and neither role holds the admin wildcard.
func TestDemoRoleDefinitionsStayWithinTheirIntent(t *testing.T) {
	t.Parallel()

	definitions := demoRoleDefinitions()
	require.Len(t, definitions, 2)
	caregiver, lead := definitions[0], definitions[1]
	assert.Equal(t, demoRoleCaregiverName, caregiver.Name)
	assert.Equal(t, demoRoleLeadName, lead.Name)

	for _, definition := range definitions {
		seen := map[string]bool{}
		for _, name := range definition.Permissions {
			assert.False(t, seen[name], "%s lists %s twice", definition.Name, name)
			seen[name] = true
			assert.NotContains(t, []string{"admin:*", "*:*", "system:manage", "auth:manage"}, name, "%s must not be an administrator", definition.Name)
			assert.False(t, strings.HasPrefix(name, "roles:") || strings.HasPrefix(name, "permissions:") || strings.HasPrefix(name, "iot:"),
				"%s must leave system administration to all functions, got %s", definition.Name, name)
		}
	}
	for _, name := range caregiver.Permissions {
		assert.Contains(t, lead.Permissions, name, "the lead keeps what the caregiver has")
	}
	assert.NotContains(t, caregiver.Permissions, "users:create", "creating children is administration")
	assert.Contains(t, lead.Permissions, "config:manage", "enrollment, settings and the planning group hang on it")
}
