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

// fileStoreSettings is the settings write that switches staff uploads.
type fileStoreSettings interface {
	SetValue(ctx context.Context, key string, value any, changedBy *int64, userPermissions []string) error
}

func setupFileStoreModule(t *testing.T, objects api.UploadsBackend) (*bun.DB, *filestorageModule.Module, fileStoreSettings) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	files, settings, err := api.NewFileStoreTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), objects)
	require.NoError(t, err)
	return db, files, settings
}
