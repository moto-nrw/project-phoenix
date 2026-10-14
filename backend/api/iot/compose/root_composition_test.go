package compose_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupActiveModule(t *testing.T) (*bun.DB, api.ActiveTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewActiveTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func setupAuthModule(t *testing.T) (*bun.DB, api.AuthTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func setupFeedbackModule(t *testing.T) (*bun.DB, api.FeedbackTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func setupIoTDataModule(t *testing.T) (*bun.DB, api.IoTDataTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewIoTDataTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func setupTimetableModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, api.TimetableTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewTimetableTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}
