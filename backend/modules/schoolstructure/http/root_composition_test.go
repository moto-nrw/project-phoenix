package schoolstructurehttp_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupGroupsModule(t *testing.T, publishers ...*testpkg.RecordingBroadcaster) (*bun.DB, api.GroupsTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	var module api.GroupsTestModule
	var err error
	if len(publishers) > 0 && publishers[0] != nil {
		module, err = api.NewGroupsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), publishers[0])
	} else {
		module, err = api.NewGroupsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	}
	require.NoError(t, err)
	return db, module
}
