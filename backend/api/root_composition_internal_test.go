package api

import (
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupClassListModule(t *testing.T) (*bun.DB, ClassListTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := NewClassListTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	return db, module
}

func setupGuardianModule(t *testing.T) (*bun.DB, GuardianTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := NewGuardianTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func setupSettingsCallbacksModule(t *testing.T) (*bun.DB, SettingsCallbacksTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := NewSettingsCallbacksTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), newStudentPhotoTestBootstrap().Unlinker)
	require.NoError(t, err)
	return db, module
}

func setupStaffModule(t *testing.T) (*bun.DB, StaffTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := NewStaffTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func setupStudentModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, StudentTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	feedback, err := NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	module, err := NewStudentTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), feedback.Feedback, clocks...)
	require.NoError(t, err)
	return db, module
}
