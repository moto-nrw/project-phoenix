package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// permissionCatalogPath is the shipped permission catalog, relative to the
// backend root: every permission the migrations create in auth.permissions.
//
// It exists because the catalog has no single place in code — it is assembled
// by dozens of migrations — while two consumers need to see it whole. This
// test pins it against a migrated database; the tenant portal checks that
// every entry has German labels
// (frontend/src/lib/permission-labels.test.ts, issue #3238). Without that
// pairing a new permission migration silently shipped an English description
// and a raw key into the Berechtigungen list.
//
// It is not imported from Go on purpose: the architecture policy keeps
// test-support out of security-runtime/contract, and the catalog is data, not
// behavior.
const permissionCatalogPath = "auth/authorize/permissions/catalog.json"

type permissionCatalogEntry struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

// name is the permission name as stored in auth.permissions — the column is
// always "resource:action".
func (e permissionCatalogEntry) name() string {
	return e.Resource + ":" + e.Action
}

func loadPermissionCatalog(t *testing.T) []permissionCatalogEntry {
	t.Helper()

	backendRoot, err := findBackendRoot()
	require.NoError(t, err)

	raw, err := os.ReadFile(filepath.Join(backendRoot, permissionCatalogPath))
	require.NoError(t, err)

	var entries []permissionCatalogEntry
	require.NoError(t, json.Unmarshal(raw, &entries))
	require.NotEmpty(t, entries)
	for i, entry := range entries {
		require.NotEmpty(t, entry.Resource, "catalog entry %d has no resource", i)
		require.NotEmpty(t, entry.Action, "catalog entry %d has no action", i)
	}
	return entries
}

// TestPermissionCatalogMatchesMigratedDatabase fails when a migration adds,
// renames or removes a permission without the catalog following. Adding a
// permission means adding it to the catalog too — that is the point, not an
// inconvenience: the frontend test then demands the German wording.
func TestPermissionCatalogMatchesMigratedDatabase(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	ctx := context.Background()

	var stored []string
	require.NoError(t, db.NewRaw(
		`SELECT name FROM auth.permissions ORDER BY name`,
	).Scan(ctx, &stored))
	require.NotEmpty(t, stored, "migrated database must contain permissions")

	catalog := loadPermissionCatalog(t)
	expected := make([]string, 0, len(catalog))
	for _, entry := range catalog {
		expected = append(expected, entry.name())
	}

	assert.ElementsMatch(t, expected, stored,
		"backend/"+permissionCatalogPath+" is out of sync with the migrations. "+
			"Add the new permission to the catalog and give it German labels in "+
			"frontend/src/lib/permission-labels.ts (#3238).")
}

// TestPermissionCatalogNamesAreUnique guards the catalog file itself: a
// duplicated entry would still match the database set-wise in a careless
// comparison, and it would hide a copy-paste mistake from the frontend check.
func TestPermissionCatalogNamesAreUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{})
	for _, entry := range loadPermissionCatalog(t) {
		name := entry.name()
		_, duplicate := seen[name]
		assert.False(t, duplicate, "permission %s is listed twice in the catalog", name)
		seen[name] = struct{}{}
	}
}
