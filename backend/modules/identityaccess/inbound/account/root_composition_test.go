package account_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: the setup helpers
// open the package's test database and compose the module through the root,
// the others work on the database their caller opened (#4005).

func parentPortalChildIDs(t *testing.T, ctx context.Context, db *bun.DB, module api.StudentTestModule, live api.ActiveTestModule, accountID int64) []int64 {
	t.Helper()
	portal, err := api.NewParentCareScheduleTestService(testpkg.NewDatabase(t, db), module, live, api.NewActiveTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db)))
	require.NoError(t, err)
	children, err := portal.ListChildrenForAccount(ctx, accountID)
	require.NoError(t, err)
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.StudentID)
	}
	return ids
}

func setupAuthModule(t *testing.T) (*bun.DB, api.AuthTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func setupAuthModuleOn(t *testing.T, db *bun.DB, options ...api.AuthTestOption) api.AuthTestModule {
	t.Helper()
	module, err := api.NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), options...)
	require.NoError(t, err)
	return module
}

func setupInvitationModule(t *testing.T) (*bun.DB, api.InvitationTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewInvitationTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func setupStudentModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, api.StudentTestModule, api.ActiveTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	feedback, err := api.NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	module, live, _, _, _, _, err := api.NewStudentTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), feedback.Feedback, clocks...)
	require.NoError(t, err)
	return db, module, live
}

func storeRawTenantSetting(t *testing.T, db *bun.DB, tenantID int64, key string, raw json.RawMessage) {
	t.Helper()
	ctx := testpkg.TenantContext(tenantID)
	require.NoError(t, testpkg.TenantCommand4(t, ctx, db, api.StoreRawTenantSettingForTests, testpkg.NewDatabase(t, db), tenantID, key, raw),
		"store raw %s override", key)
}

func submitParentCareScheduleRequest(t *testing.T, ctx context.Context, db *bun.DB, module api.StudentTestModule, live api.ActiveTestModule, accountID, studentID int64, payload map[string]any) (requestID int64, err error) {
	t.Helper()
	portal, composeErr := api.NewParentCareScheduleTestService(testpkg.NewDatabase(t, db), module, live, api.NewActiveTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db)))
	require.NoError(t, composeErr)
	view, err := portal.CreateCareScheduleRequest(ctx, accountID, studentID, payload)
	if err != nil {
		return 0, err
	}
	require.NotNil(t, view.PendingRequest)
	return view.PendingRequest.ID, nil
}
