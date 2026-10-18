package presenceprojectionhttp_test

import (
	"context"
	"errors"
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
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	studentsAPI "github.com/moto-nrw/project-phoenix/modules/presenceprojection/http"
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
type settingsWriter interface {
	SetValue(context.Context, string, any, *int64, []string) error
	ResetValue(context.Context, string, *int64, []string) error
}

func (tc *testContext) settings() settingsWriter { return tc.settingsService.(settingsWriter) }
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
	db := testpkg.SetupTestDB(t)
	feedback, err := api.NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc, live, people, audit, decisions, groupLive, err := api.NewStudentHTTPTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), feedback.Feedback, broadcaster, clocks...)
	require.NoError(t, err)
	liveEducation := api.NewActiveTestEducation(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	livePeople := api.BindStudentTestPeople(api.NewActiveTestPeople(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...), svc)
	livePresence := api.NewActiveTestPresence(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	liveTimetable := api.NewActiveTestTimetable(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	liveCaller := api.NewActiveTestCaller(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	liveSettings := api.NewActiveTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	liveDevices := api.NewActiveTestDevices(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	_, _, resource, _, _, _, mount, err := api.NewStudentHTTPTestResources(
		testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), svc, live, liveSettings, liveEducation, liveCaller, livePeople, livePresence, liveTimetable, people, audit, groupLive, broadcaster, tenantScopedCaller{liveCaller},
		testpkg.StudentDirectory{}, &testpkg.FeedbackEntryCounterMock{}, testpkg.RequestAuditActor,
		deviceAuthenticator(t, db, liveDevices, liveSettings), testutil.AuthenticatedDeviceID, authoritative, firstClock(clocks),
	)
	require.NoError(t, err)
	router := func() chi.Router { return mount(resource.Now) }
	return &testContext{careRequests: svc.CareRequests, enrollmentDecision: decisions, db: db, resource: resource,
		settingsService: liveSettings.Settings, broadcaster: broadcaster, clock: firstClock(clocks), newPickupAdjustments: svc.NewPickupAdjustments, module: svc, router: router}
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

const studentsTestToday calendar.Date = "2026-08-24"

func fixedCalendarClock() time.Time { return studentsTestToday.BerlinMidnight().Add(12 * time.Hour) }
func authExec(t *testing.T, tc *testContext, req *http.Request, claims sessiontoken.AppClaims, perms []string) *httptest.ResponseRecorder {
	t.Helper()
	claims.Permissions = perms
	req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	return testutil.ExecuteRequestForTest(t, tc.router(), req)
}

// readStudentRecord reads the child's owned row with its departure plan in
// the effective form the routes hydrate it to.
func readStudentRecord(t *testing.T, tc *testContext, studentID int64) (peopleModule.StudentRecord, error) {
	t.Helper()
	record, err := tc.resource.PeopleDirectory.FindStudentRecord(testpkg.Ctx(t), studentID)
	if err != nil {
		return record, err
	}
	effective := departure.Plan{
		AllowedDepartureModes: record.AllowedDepartureModes,
		DepartureDays:         record.DepartureDays,
		BusDays:               record.BusDays,
		PickupDays:            record.PickupDays,
	}.Effective()
	record.AllowedDepartureModes = effective.AllowedDepartureModes
	record.DepartureDays = effective.DepartureDays
	record.BusDays = effective.BusDays
	record.PickupDays = effective.PickupDays
	return record, nil
}

// deviceAuthenticator is the kiosk device authentication over the module's
// fleet and settings, with the root's guard against deleted schools.
func deviceAuthenticator(t testing.TB, db *bun.DB, live api.DeviceTestModule, liveSettings api.ActiveSettings) common.Middleware {
	t.Helper()
	schools, err := api.NewDeviceSchools(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return testutil.NewDeviceAuthenticators(live.IoT.Fleet(), schools, liveSettings.Settings, testDevicePIN).Device()
}

const testDevicePIN = "1234"
