package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	uploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
	compose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentHTTP "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	failures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// Counts include every SQL statement issued by the actual mounted handler,
// including its transaction envelope. JWT verification itself performs no SQL.
// Settings, public school resolution and catalog reads use real persistence.
func TestEnrollmentCatalogRouteGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	f := newEnrollmentGrowthFixture(t)
	env, requests, catalog, router, token, slug := f.env, f.resource.RequestService, f.resource.CareOfferingService, f.router(t), f.token, f.slug
	ctx := testpkg.Ctx(t)
	phaseID := strconv.FormatInt(env.phaseID, 10)
	type route struct {
		path, scenario, collection string
		list                       bool
	}
	routes := []route{
		{"/care-offerings/", "api.enrollment.growth.care_offerings", "", true},
		{"/care-offerings/booking-stats?phase_id=" + phaseID, "api.enrollment.growth.booked_stats", "", true},
		{"/care-offerings/public/" + slug + "/" + phaseID, "api.enrollment.growth.public_offerings", "offerings", false},
		{"/form-bootstrap/public/" + slug + "/" + phaseID, "api.enrollment.growth.public_bootstrap", "offerings", false},
		{"/phases/" + phaseID + "/manual-bootstrap", "api.enrollment.growth.manual_bootstrap", "offerings", false},
	}
	counter := testpkg.CaptureQueries(t, env.db)
	var ids []string
	add := func(n int) {
		for range n {
			i := len(ids)
			o, createErr := testpkg.TenantQuery1(t, ctx, env.db, catalog.Create, &enrollmentHTTP.CareOffering{PhaseID: env.phaseID, Name: fmt.Sprintf("Budget %02d", i), DaysOfWeekMode: "fixed", AvailableDays: []string{"mon"}, PickupTimes: map[string]string{"mon": "15:00"}, IsActive: true, SortOrder: i})
			require.NoError(t, createErr)
			ids = append(ids, strconv.FormatInt(o.ID, 10))
			request := makeOwnerRequest(env.phaseID, uniqueToken("catalog-booking-growth"), fmt.Sprintf("booking%d@example.test", i))
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertRequest, request))
			student := testpkg.CreateTestStudent(t, env.db, "Booked", fmt.Sprintf("Child%d", i), "1a")
			child := makeChild(request.ID, "Booked", "Child")
			child.Status = enrollment.ChildStatusApproved
			child.CreatedStudentID = &student.ID
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChild, child))
			require.NoError(t, api.NewEnrollmentBookingFixtureWithin(testpkg.WithinCurrentTenant).InsertRequestChildOffering(ctx, &enrollment.RequestChildOffering{RequestChildID: child.ID, CareOfferingID: o.ID, SelectedDays: []string{"mon"}}))
		}
	}
	run := func(r route, n int) testpkg.QueryMeasurement {
		req := httptest.NewRequest(http.MethodGet, r.path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		counter.Reset()
		testpkg.MeasuredHTTPRequest(counter, testpkg.MountedQueryBudgetRouter("/api/enrollment", router), response, req)
		queries := counter.Queries()
		require.Equal(t, http.StatusOK, response.Code, "%s: %s", r.path, response.Body.String())
		var body struct {
			Data json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		raw := body.Data
		if !r.list {
			var object map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &object))
			raw = object[r.collection]
		}
		var rows []struct {
			ID         string `json:"id"`
			OfferingID string `json:"offering_id"`
			Name       string `json:"name"`
			Booked     int    `json:"booked"`
		}
		require.NoError(t, json.Unmarshal(raw, &rows))
		require.Len(t, rows, n, r.path)
		for i, row := range rows {
			id := row.ID
			if id == "" {
				id = row.OfferingID
			}
			require.Equal(t, ids[i], id, r.path)
			if r.scenario == "api.enrollment.growth.booked_stats" {
				require.Equal(t, 1, row.Booked)
			}
		}
		return counter.Measurement(queries)
	}
	add(3)
	small := make([]testpkg.QueryMeasurement, len(routes))
	for i, r := range routes {
		small[i] = run(r, 3)
	}
	add(5)
	for i, r := range routes {
		large := run(r, 8)
		require.Equal(t, len(small[i].Statements), len(large.Statements), r.path)
		t.Logf("%s N3=%d N8=%d", r.scenario, len(small[i].Statements), len(large.Statements))
	}
	// Status and edit bootstrap grow the sibling collection with a nonempty catalog.
	requestReads := func(n int) []testpkg.QueryMeasurement {
		input := validSubmission(t, env.phaseID)
		input.GuardianEmail = fmt.Sprintf("growth%d@example.test", n)
		input.Children = nil
		for i := 0; i < n; i++ {
			child := validSubmission(t, env.phaseID).Children[0]
			child.FirstName = fmt.Sprintf("Child%d", i)
			input.Children = append(input.Children, child)
		}
		submitted, submitErr := requests.Submit(ctx, input)
		require.NoError(t, submitErr)
		require.NotEmpty(t, submitted.Request.StatusToken)
		result := make([]testpkg.QueryMeasurement, 2)
		for i, suffix := range []string{"", "/edit-bootstrap"} {
			path := "/requests/" + submitted.Request.StatusToken + suffix
			counter.Reset()
			rec := httptest.NewRecorder()
			testpkg.MeasuredHTTPRequest(counter, testpkg.MountedQueryBudgetRouter("/api/enrollment", // Production mounts the request-scoped settings cache router-wide
				// (api/serve_middleware.go); the token pages count with it, so a
				// further school setting costs no statement of its own (#6042).
				api.RequestSettingsCacheMiddleware(router)), rec, httptest.NewRequest(http.MethodGet, path, nil))
			result[i] = counter.Measurement(counter.Queries())
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var body struct {
				Data map[string]json.RawMessage `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			object := body.Data
			if i == 1 {
				require.NoError(t, json.Unmarshal(object["draft"], &object))
			}
			var children []struct {
				FirstName string `json:"first_name"`
			}
			require.NoError(t, json.Unmarshal(object["children"], &children))
			require.Len(t, children, n)
			for j, child := range children {
				require.Equal(t, fmt.Sprintf("Child%d", j), child.FirstName)
			}
		}
		return result
	}
	requestSmall := requestReads(3)
	requestLarge := requestReads(8)
	for i := range requestSmall {
		require.Equal(t, len(requestSmall[i].Statements), len(requestLarge[i].Statements))
		t.Logf("request route %d N3=%d N8=%d", i, len(requestSmall[i].Statements), len(requestLarge[i].Statements))
	}
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.status", requestLarge[0])
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.edit_bootstrap", requestLarge[1])
	// Literal references keep each authored contract bound to this test.
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.care_offerings", run(routes[0], 8))
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.booked_stats", run(routes[1], 8))
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.public_offerings", run(routes[2], 8))
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.public_bootstrap", run(routes[3], 8))
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.manual_bootstrap", run(routes[4], 8))
}

type enrollmentGrowthFixture struct {
	env         *requestTestEnv
	resource    *enrollmentHTTP.Resource
	token, slug string
	settings    owners.SettingsTestModule
}

func newEnrollmentGrowthFixture(t *testing.T) *enrollmentGrowthFixture {
	t.Helper()
	testpkg.SetupIsolatedTestDB(t)
	env := setupRequestTest(t)
	settings, err := owners.NewSettingsTestModule(testpkg.NewDatabase(t, env.db), testpkg.TenantRuntime(t, env.db))
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	for _, key := range []string{settingKeyEnrollmentEnabled, settingKeyAllowSubmissionEdit, settingKeyCareOfferingsEnabled} {
		require.NoError(t, testpkg.TenantCommand4(t, ctx, env.db, settings.Settings.SetValue, key, any(true), &env.creatorID, []string{"config:update"}))
	}
	cfg := env.config
	cfg.Settings = settings.Settings
	requests := newTestRequestService(cfg)
	catalog := enrollmentHTTP.NewCareOfferingCatalog(api.NewEnrollmentCareOfferingValues(newCareOfferingCatalog(t, env.db).Catalog))
	phases := NewTestPhases(TestPhaseDependencies{Records: env.repos.Enrollment(), Offerings: phaseOfferings{rows: newCareOfferingFixtures(env.reposCare.CarePlan)}, LockTemplateRecurrence: func(context.Context) error { return nil }, Resyncer: &recordingSourcedTemplateResyncer{}, Settings: collectionSettings{settings: settings.Settings}})
	resource := enrollmentHTTP.NewResource(compose.NewFormSchemas(env.repos.Enrollment(), collectionSettings{settings: settings.Settings}, nil), catalog, requests, compose.NewCaptcha(owners.EnrollmentCaptchaSettings{Settings: settings.Settings}, "", nil), phases, nil, nil, nil, nil, nil, enrollmentHTTP.GuardianInvitationRuntime{}, nil, dbPublicSchools{db: env.db}, uploads.UploadFiles{}, enrollmentHTTP.DatabaseFailures{Transient: failures.IsTransientDatabaseError, Constraint: failures.IsConstraintViolation})
	account := testpkg.CreateTestAccount(t, env.db, "enrollment-growth")
	token := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Username: account.Email, TenantID: testpkg.Tenant(t), Roles: []string{"admin"}, Permissions: []string{"config:read", "config:manage"}})
	var slug string
	require.NoError(t, env.db.NewSelect().TableExpr("platform.schools").Column("slug").Where("id = ?", testpkg.Tenant(t)).Scan(ctx, &slug))
	require.NotEmpty(t, slug)
	return &enrollmentGrowthFixture{env: env, resource: resource, token: token, slug: slug, settings: settings}
}
func (f *enrollmentGrowthFixture) router(t *testing.T) http.Handler {
	t.Helper()
	return testpkg.TenantRuntimeMiddleware(t, f.env.db)(f.resource.Router(api.TenantSessionChain))
}
