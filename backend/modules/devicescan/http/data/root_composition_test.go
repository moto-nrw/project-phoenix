package data_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func composeFeedbackModule(t *testing.T) (*bun.DB, api.FeedbackTestModule, owners.SettingsTestModule, api.RFIDTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, settings, err := api.NewFeedbackTestModuleWithSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	identity, err := api.NewRFIDTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	return db, module, settings, identity
}

func setupIoTDataModule(t *testing.T) (*bun.DB, api.IoTDataTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, _, _, _, _, _, _, err := api.NewIoTDataTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func composeRFIDModule(t *testing.T) (*bun.DB, api.RFIDTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewRFIDTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	return db, module
}
