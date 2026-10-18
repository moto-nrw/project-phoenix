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

func setupStudentModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, StudentTestModule, ActiveTestModule, ActiveSettings, *CallerRows) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	feedback, err := NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	module, live, _, _, _, _, err := NewStudentTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), feedback.Feedback, clocks...)
	require.NoError(t, err)
	liveSettings := NewActiveTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	liveCaller := NewActiveTestCaller(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	return db, module, live, liveSettings, liveCaller
}
