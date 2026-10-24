package enrollmenthttp_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	enrollmentHTTP "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type studentExtraFieldsTestContext struct {
	db       *testpkg.DB
	resource *enrollmentHTTP.StudentResource
}

func setupStudentsRoute(t *testing.T) *studentExtraFieldsTestContext {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	feedback, err := api.NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	_, _, people, _, _, _, err := api.NewStudentTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), feedback.Feedback)
	require.NoError(t, err)
	liveCaller := api.NewActiveTestCaller(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	return &studentExtraFieldsTestContext{db: db, resource: api.NewStudentEnrollmentHTTPTestResource(people, liveCaller)}
}
func authExec(t *testing.T, tc *studentExtraFieldsTestContext, req *http.Request, claims testutil.Claims, perms []string) *httptest.ResponseRecorder {
	t.Helper()
	claims.Permissions = perms
	req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	r := api.NewOwnerHTTPTestRouter(tc.resource.Register, "")
	return testutil.ExecuteRequestForTest(t, r, req)
}
