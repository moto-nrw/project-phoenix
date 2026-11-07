//go:build testhooks

package httpadapter_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	activitiesHTTP "github.com/moto-nrw/project-phoenix/modules/timetable/http/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Only the two failure seams differ from the real service. All other
// dependencies and the protected public router retain their production wiring.
type categoryListFailureService struct {
	activityContract.ActivityService
	list  func(context.Context) ([]*activityContract.Category, error)
	usage func(context.Context) (map[int64]int, error)
}

func (s categoryListFailureService) ListCategories(ctx context.Context) ([]*activityContract.Category, error) {
	return s.list(ctx)
}

func (s categoryListFailureService) CategoryUsageCounts(ctx context.Context) (map[int64]int, error) {
	return s.usage(ctx)
}

func TestActivityCategoriesListFailurePaths(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, scenario := range []struct {
		name           string
		visible        bool
		hidden         bool
		listError      bool
		usageError     bool
		cancelBefore   bool
		cancelAfter    bool
		wantListCalls  int
		wantUsageCalls int
	}{
		{name: "list error", listError: true, wantListCalls: 1},
		{name: "usage error with visible category", visible: true, usageError: true, wantListCalls: 1, wantUsageCalls: 1},
		{name: "cancel before transaction", cancelBefore: true},
		{name: "cancel after empty list", cancelAfter: true, wantListCalls: 1, wantUsageCalls: 1},
		{name: "cancel after hidden list", hidden: true, cancelAfter: true, wantListCalls: 1, wantUsageCalls: 1},
		{name: "cancel after visible list", visible: true, cancelAfter: true, wantListCalls: 1, wantUsageCalls: 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			if scenario.visible {
				testpkg.CreateTestActivityCategory(t, db, "Visible failure fixture")
			}
			if scenario.hidden {
				archived := time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC)
				for _, category := range []testpkg.ActivityCategory{
					{TenantID: testpkg.Tenant(t), Name: "Hidden system", IsSystem: true},
					{TenantID: testpkg.Tenant(t), Name: "Hidden archived", ArchivedAt: &archived},
				} {
					_, err := db.NewInsert().Model(&category).Exec(testpkg.Ctx(t))
					require.NoError(t, err)
				}
			}
			module, service, people, caller, err := api.NewActivitiesTestModule(testpkg.NewDatabase(t, db))
			require.NoError(t, err)
			request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/api/activities/categories?with_usage=true", nil)
			ctx, cancel := context.WithCancel(request.Context())
			defer cancel()
			request = request.WithContext(ctx)
			listCalls, usageCalls := 0, 0
			probe := categoryListFailureService{
				ActivityService: service,
				list: func(ctx context.Context) ([]*activityContract.Category, error) {
					listCalls++
					if scenario.listError {
						return nil, &activityContract.ActivityError{Op: "list categories", Err: errors.New("category read failed")}
					}
					rows, err := service.ListCategories(ctx)
					require.NoError(t, err)
					if scenario.cancelAfter {
						cancel()
					}
					return rows, nil
				},
				usage: func(ctx context.Context) (map[int64]int, error) {
					usageCalls++
					if scenario.usageError {
						return nil, &activityContract.ActivityError{Op: "category usage counts", Err: errors.New("usage read failed")}
					}
					return service.CategoryUsageCounts(ctx)
				},
			}
			resource := activitiesHTTP.NewResource(probe, module.Timetable, api.NewTimetableSupervisorDirectory(people), caller, api.TimetableHTTPTransactions)
			router := chi.NewRouter()
			router.Mount("/api/activities", resource.Router(api.ProtectedTenantRoutes))
			if scenario.cancelBefore {
				cancel()
			}
			counter := testpkg.CaptureQueries(t, db)
			response := testutil.ExecuteWithAuth(t, router, request, caregiverClaims())
			logCatalogQueries(t, scenario.name, 0, counter.Queries())
			t.Logf("response=%s write_rows=%d", response.Body.String(), counter.WriteRows())
			require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
			body := testutil.ParseJSONResponse(t, response.Body.Bytes())
			require.Equal(t, "general.server", body["code"])
			require.Equal(t, float64(http.StatusInternalServerError), body["status"])
			require.NotContains(t, body, "data", "failed/cancelled reads must never publish a buffered success")
			if scenario.cancelAfter {
				require.Equal(t, "activity error during category usage counts: timetable postgres: count category usage: context canceled", body["detail"], "retain the original usage failure instead of a generic commit failure")
			}
			require.Equal(t, scenario.wantListCalls, listCalls)
			require.Equal(t, scenario.wantUsageCalls, usageCalls)
			assertCatalogReadOnly(t, counter)
			counter.Stop()
		})
	}
}

func TestActivityCategoriesListRejectsWrongPortalAndMissingTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	router, _, _ := activitiesCatalogBudgetRouter(t, db)
	counter := testpkg.CaptureQueries(t, db)
	for _, scope := range []string{"parent", "school", "org", "caterer", "missing tenant"} {
		t.Run(scope, func(t *testing.T) {
			claims := caregiverClaims()
			claims.Scope = scope
			if scope == "missing tenant" {
				claims.Scope, claims.TenantID = "", 0
			}
			for _, flags := range []string{"", "&include_system=true&include_archived=true"} {
				request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/api/activities/categories?with_usage=true"+flags, nil, testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)))
				counter.Reset()
				response := testutil.ExecuteRequestForTest(t, router, request)
				require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
				require.Equal(t, "general.permission", testutil.ParseJSONResponse(t, response.Body.Bytes())["code"])
				require.Empty(t, counter.Queries(), "scope rejection must happen before opening a transaction")
				assertCatalogReadOnly(t, counter)
			}
		})
	}
}
