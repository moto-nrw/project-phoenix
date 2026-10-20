package files_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

// setupFileStoreModule returns the module with the tenant setting switch
// the suites turn staff uploads on with.
func setupFileStoreModule(t *testing.T, objects api.UploadsBackend) (*bun.DB, filestorageModule.Capability, func(context.Context, string, bool) error) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	files, settings, err := api.NewFileStoreTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), objects)
	require.NoError(t, err)
	setSetting := func(ctx context.Context, key string, value bool) error {
		return settings.SetValue(ctx, key, value, nil, nil)
	}
	return db, files, setSetting
}
