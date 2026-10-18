package grouplivehttp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	groupLiveHTTP "github.com/moto-nrw/project-phoenix/modules/grouplive/http"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type settingsWriter interface {
	SetValue(context.Context, string, any, *int64, []string) error
	ResetValue(context.Context, string, *int64, []string) error
}
type testContext struct {
	db              *testpkg.DB
	resource        *groupLiveHTTP.Resource
	settingsService interface {
		ResolveBool(context.Context, string) (bool, error)
		ResolveInt(context.Context, string) (int, error)
		ResolveString(context.Context, string) (string, error)
	}
}

func (tc *testContext) settings() settingsWriter { return tc.settingsService.(settingsWriter) }
func setupStudentsRoute(t *testing.T, clocks ...func() time.Time) *testContext {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	feedback, err := api.NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	_, live, _, _, _, groupLive, err := api.NewStudentTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), feedback.Feedback, clocks...)
	require.NoError(t, err)
	return &testContext{db: db, resource: groupLiveHTTP.NewResource(groupLive), settingsService: live.Settings}
}
func authExec(t *testing.T, tc *testContext, req *http.Request, claims testutil.Claims, perms []string) *httptest.ResponseRecorder {
	t.Helper()
	claims.Permissions = perms
	req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	r := api.NewOwnerHTTPTestRouter(tc.resource.Register, "")
	return testutil.ExecuteRequestForTest(t, r, req)
}
func TestMain(m *testing.M) { testpkg.PerTestTenants(); testutil.SeedTestJWTConfig(); testpkg.Run(m) }

func fixedCalendarClock() time.Time {
	return calendar.Date("2026-08-24").BerlinMidnight().Add(12 * time.Hour)
}
