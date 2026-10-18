package studentdeletionhttp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	studentsAPI "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/http"
)

// testContext retains the test's subject and its existing fixture module.
type testContext struct {
	careRequests       carerequests.Submissions
	enrollmentDecision enrollmentOwner.Decisions
	db                 *bun.DB
	resource           *studentsAPI.Resource
	settingsService    interface {
		ResolveBool(context.Context, string) (bool, error)
		ResolveInt(context.Context, string) (int, error)
		ResolveString(context.Context, string) (string, error)
	}
	broadcaster          *testpkg.RecordingBroadcaster
	clock                func() time.Time
	newPickupAdjustments func(careplan.DirectOfferingAdjustments) (careplan.PickupAdjustments, error)
	module               api.StudentTestModule
	router               func() chi.Router
}

func setupStudentsRoute(t *testing.T, clocks ...func() time.Time) *testContext {
	t.Helper()
	return newStudentsRoute(t, nil, clocks...)
}
func setupStudentsRouteWithCareLifecycle(t *testing.T, authoritative bool, clocks ...func() time.Time) *testContext {
	t.Helper()
	return newStudentsRoute(t, &authoritative, clocks...)
}
func newStudentsRoute(t *testing.T, authoritative *bool, clocks ...func() time.Time) *testContext {
	t.Helper()
	db, svc := setupStudentModule(t, clocks...)
	broadcaster := testpkg.NewRecordingBroadcaster()
	_, _, _, _, resource, _, router, err := api.NewStudentHTTPTestResources(
		testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), svc, broadcaster, tenantScopedCaller{svc.UserContext},
		testpkg.StudentDirectory{}, &testpkg.FeedbackEntryCounterMock{}, testpkg.RequestAuditActor,
		deviceAuthenticator(t, db, svc), testutil.AuthenticatedDeviceID, authoritative, firstClock(clocks),
	)
	require.NoError(t, err)
	return &testContext{careRequests: svc.CareRequests, enrollmentDecision: svc.EnrollmentDecision, db: db, resource: resource,
		settingsService: svc.Settings, broadcaster: broadcaster, clock: firstClock(clocks), newPickupAdjustments: svc.NewPickupAdjustments, module: svc, router: router}
}

type tenantScopedCaller struct{ *api.CallerRows }

func (c tenantScopedCaller) HasCurrentStaff(ctx context.Context) (bool, error) {
	if _, ok := testpkg.TransactionFromContext(ctx); !ok {
		return false, errors.New("staff lookup outside a tenant transaction")
	}
	return c.CallerRows.HasCurrentStaff(ctx)
}
func firstClock(clocks []func() time.Time) func() time.Time {
	if len(clocks) == 0 {
		return nil
	}
	return clocks[0]
}

func authExec(t *testing.T, tc *testContext, req *http.Request, claims sessiontoken.AppClaims, perms []string) *httptest.ResponseRecorder {
	t.Helper()
	claims.Permissions = perms
	req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	return testutil.ExecuteRequestForTest(t, tc.router(), req)
}

// newStudentTestRepositories composes the retained repositories the suites
// arrange through, recording into the test audit store.
var newStudentTestRepositories = api.MustNewStudentHTTPTestRows

// previewStudentDeletion reads the delete-impact preview the confirmed
// deletion has to quote back (#2710): the permanent deletion is never
// reachable without it.
func previewStudentDeletion(t *testing.T, tc *testContext, claims sessiontoken.AppClaims, studentID int64) map[string]any {
	t.Helper()
	request := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/%d/delete-impact", studentID), nil)
	response := authExec(t, tc, request, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, response.Code, "Body: %s", response.Body.String())
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	return body.Data
}

// confirmStudentDeletion sends the confirmed DELETE for a preview.
func confirmStudentDeletion(t *testing.T, tc *testContext, claims sessiontoken.AppClaims, studentID int64, preview map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	request := testutil.NewAuthenticatedRequest(t, http.MethodDelete, fmt.Sprintf("/%d", studentID), map[string]any{
		"expected_fingerprint": preview["fingerprint"],
		"confirmation_name":    preview["confirmation_name"],
		"reason":               "test_data",
		"acknowledged":         true,
	})
	return authExec(t, tc, request, claims, []string{"admin:*"})
}

// deleteStudentConfirmed previews and confirms in one step for tests whose
// subject is the deletion's side effect, not the confirmation itself.
func deleteStudentConfirmed(t *testing.T, tc *testContext, claims sessiontoken.AppClaims, studentID int64) *httptest.ResponseRecorder {
	t.Helper()
	return confirmStudentDeletion(t, tc, claims, studentID, previewStudentDeletion(t, tc, claims, studentID))
}

// deviceAuthenticator is the kiosk device authentication over the module's
// fleet and settings, with the root's guard against deleted schools.
func deviceAuthenticator(t testing.TB, db *bun.DB, svc api.StudentTestModule) common.Middleware {
	t.Helper()
	schools, err := api.NewDeviceSchools(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return testutil.NewDeviceAuthenticators(svc.IoT.Fleet(), schools, svc.Settings, testDevicePIN).Device()
}

func setupStudentModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, api.StudentTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	feedback, err := api.NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	module, err := api.NewStudentTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), feedback.Feedback, clocks...)
	require.NoError(t, err)
	return db, module
}

const testDevicePIN = "1234"
