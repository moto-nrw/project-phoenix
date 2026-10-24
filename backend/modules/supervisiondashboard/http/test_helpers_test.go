package supervisiondashboardhttp_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	dashboardHTTP "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type testSettingsWriter struct {
	SetString  func(context.Context, string, string, *int64, []string) error
	SetBool    func(context.Context, string, bool, *int64, []string) error
	ResetValue func(context.Context, string, *int64, []string) error
}
type testContext struct {
	db       *testpkg.DB
	settings testSettingsWriter
}

func newDashboardContext(t *testing.T) (*testContext, testutil.Router) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	_, err := api.NewActiveTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	svcSupervision := api.NewActiveTestSupervision(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	svcSettings := api.NewActiveTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	resource := dashboardHTTP.NewResource(svcSupervision.SupervisionDashboard)
	router := api.NewOwnerHTTPTestRouter(resource.Register, "/active")
	return &testContext{db: db, settings: testSettingsWriter{
		SetString: func(ctx context.Context, key, value string, actor *int64, perms []string) error {
			return svcSettings.Settings.SetValue(ctx, key, value, actor, perms)
		},
		SetBool: func(ctx context.Context, key string, value bool, actor *int64, perms []string) error {
			return svcSettings.Settings.SetValue(ctx, key, value, actor, perms)
		},
		ResetValue: svcSettings.Settings.ResetValue,
	}}, router
}
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testutil.SeedTestJWTConfig()
	testpkg.Run(m)
}
