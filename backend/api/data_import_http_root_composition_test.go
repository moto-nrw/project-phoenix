package api

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupImportModule(t *testing.T, databases ...*bun.DB) (*bun.DB, ImportTestModule, peopledirectory.Capability, schoolmembership.Capability) {
	t.Helper()
	var db *bun.DB
	if len(databases) > 0 {
		db = databases[0]
	} else {
		db = testpkg.SetupTestDB(t)
	}
	module, people, membership, err := NewImportTestModuleWithOwners(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), ImportTestOptions{})
	require.NoError(t, err)
	return db, module, people, membership
}
