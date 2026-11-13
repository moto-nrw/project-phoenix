//go:build testhooks

package httpadapter_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/timetable"
	activitiesHTTP "github.com/moto-ogs/project-phoenix/modules/timetable/http/activities"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root composes real Timetable, identity and directory owners. Only
// this resource is mounted; the protected tenant router and its transaction
// middleware are production code, without a network listener or app server.
func activitiesCatalogBudgetRouter(t *testing.T, db *bun.DB) (chi.Router, timetable.Capability, string) {
	t.Helper()
	module, activities, people, caller, err := api.NewActivitiesTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	resource := activitiesHTTP.NewResource(activities, module.Timetable, api.NewTimetableSupervisorDirectory(people), caller, api.TimetableHTTPTransactions)
	router := chi.NewRouter()
	router.Mount("/api/activities", resource.Router(api.ProtectedTenantRoutes))
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Catalog", "Reader")
	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.TenantID = testpkg.Tenant(t)
	claims.IsAdmin = false
	claims.Permissions = nil
	return router, module.Timetable, testutil.MintTestJWT(t, claims)
}

func TestActivityCategoriesListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	router, _, token := activitiesCatalogBudgetRouter(t, db)
	foreign := testpkg.NewTenantScope(t, db)
	foreignCategory := testpkg.CreateTestActivityCategoryForTenant(t, db, foreign.TenantID, "Budget category 00")
	insertCatalogUsage(t, db, foreign.TenantID, foreignCategory.ID, 5)
	archived := time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC)
	for _, hidden := range []testpkg.ActivityCategory{
		{TenantID: testpkg.Tenant(t), Name: "Hidden system", IsSystem: true},
		{TenantID: testpkg.Tenant(t), Name: "Hidden archived", ArchivedAt: &archived},
	} {
		_, err := db.NewInsert().Model(&hidden).Exec(testpkg.Ctx(t))
		require.NoError(t, err)
	}
	expected := []map[string]any{}
	counter := testpkg.CaptureQueries(t, db)
	previous := map[bool]int{}
	for _, size := range []int{0, 3, 8} {
		for len(expected) < size {
			index := len(expected)
			category := insertCatalogCategory(t, db, index)
			insertCatalogUsage(t, db, testpkg.Tenant(t), category.ID, index%3)
			expected = append(expected, map[string]any{
				"id": category.ID, "name": category.Name, "description": category.Description,
				"color": category.Color, "is_system": false, "usage_count": index % 3,
				"created_at": category.CreatedAt, "updated_at": category.UpdatedAt,
			})
		}
		sort.Slice(expected, func(i, j int) bool { return expected[i]["name"].(string) < expected[j]["name"].(string) })
		for _, withUsage := range []bool{false, true} {
			t.Run(fmt.Sprintf("n=%d/with_usage=%t", size, withUsage), func(t *testing.T) {
				want := catalogExpectedUsage(expected, withUsage)
				counter.Reset()
				request := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/api/activities/categories?with_usage=%t", withUsage), nil, testutil.WithJWTBearer(token))
				response := testutil.ExecuteRequestForTest(t, router, request)
				queries := counter.Measurement(counter.Queries())
				logCatalogQueries(t, fmt.Sprintf("categories.with_usage=%t", withUsage), size, queries.Statements)
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				assertCatalogData(t, response.Body.Bytes(), want, "Categories retrieved successfully")
				if size == 0 {
					require.Equal(t, "{\"status\":\"success\",\"data\":[],\"message\":\"Categories retrieved successfully\"}\n", response.Body.String())
				}
				assertCatalogReadOnly(t, counter)
				t.Logf("response=%s write_rows=%d", response.Body.String(), counter.WriteRows())
				require.Len(t, counter.Selects("activities.categories"), 1)
				if withUsage && size != 0 {
					require.Len(t, counter.Selects("activities.groups"), 1, "one aggregate, including archived/template references")
				} else {
					require.Empty(t, counter.Selects("activities.groups"), "pickers and empty visible lists must not read usage")
				}
				// Usage is flat across populated scopes; the empty scope skips it.
				if size == 8 || (size == 3 && !withUsage) {
					require.Len(t, queries.Statements, previous[withUsage], "statement count must not grow with N")
				}
				previous[withUsage] = len(queries.Statements)
				if withUsage && size == 0 {
					querybudgets.AssertQueryBudget(t, "api.activities.categories.empty_with_usage", queries)
				} else if withUsage {
					querybudgets.AssertQueryBudget(t, "api.activities.categories.with_usage", queries)
				} else {
					querybudgets.AssertQueryBudget(t, "api.activities.categories.without_usage", queries)
				}
			})
		}
	}
}

func insertCatalogCategory(t *testing.T, db *bun.DB, index int) testpkg.ActivityCategory {
	t.Helper()
	category := testpkg.ActivityCategory{
		TenantID: testpkg.Tenant(t), Name: fmt.Sprintf("Budget category %02d", 8-index),
		Description: fmt.Sprintf("Description %d", index), Color: "#123456",
	}
	err := db.NewInsert().Model(&category).Returning("*").Scan(testpkg.Ctx(t))
	require.NoError(t, err)
	return category
}

func insertCatalogUsage(t *testing.T, db *bun.DB, tenantID, categoryID int64, count int) {
	t.Helper()
	for index := range count {
		group := testpkg.ActivityGroup{
			TenantID: tenantID, Name: fmt.Sprintf("Usage %d", index), CategoryID: categoryID,
			MaxParticipants: 20, IsTemplate: index == 1,
		}
		if index == 0 {
			archived := time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC)
			group.ArchivedAt = &archived
		}
		_, err := db.NewInsert().Model(&group).Exec(testpkg.Ctx(t))
		require.NoError(t, err)
	}
}

func catalogExpectedUsage(expected []map[string]any, withUsage bool) []map[string]any {
	want := make([]map[string]any, 0, len(expected))
	for _, category := range expected {
		row := make(map[string]any, len(category)+1)
		for key, value := range category {
			if key != "usage_count" || withUsage {
				row[key] = value
			}
		}
		want = append(want, row)
	}
	return want
}

func TestActivityTimespansListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	router, owner, token := activitiesCatalogBudgetRouter(t, db)
	foreign := testpkg.NewTenantScope(t, db)
	_, err := owner.CreateTimeframe(foreign.Context(), timetable.TimeframeInput{StartTime: "06:00:00", IsActive: true, Description: "Foreign"})
	require.NoError(t, err)
	_, err = owner.CreateTimeframe(testpkg.Ctx(t), timetable.TimeframeInput{StartTime: "05:00:00", IsActive: false, Description: "Inactive"})
	require.NoError(t, err)
	expected := []map[string]any{}
	counter := testpkg.CaptureQueries(t, db)
	var previous int
	for _, size := range []int{0, 3, 8} {
		for len(expected) < size {
			index := len(expected)
			start := fmt.Sprintf("%02d:15:00", 16-index)
			end := fmt.Sprintf("%02d:45:00", 16-index)
			input := timetable.TimeframeInput{StartTime: start, EndTime: &end, IsActive: true, Description: fmt.Sprintf("Window %d", index)}
			wireEnd := end[:5]
			if index == 0 {
				input.EndTime, input.Description, wireEnd = nil, "", ""
			}
			created, createErr := owner.CreateTimeframe(testpkg.Ctx(t), input)
			require.NoError(t, createErr)
			row := map[string]any{"id": created.ID, "name": input.Description, "start_time": start[:5], "end_time": wireEnd}
			if input.Description != "" {
				row["description"] = input.Description
			}
			expected = append(expected, row)
		}
		t.Run(fmt.Sprintf("n=%d", size), func(t *testing.T) {
			counter.Reset()
			request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/api/activities/timespans", nil, testutil.WithJWTBearer(token))
			response := testutil.ExecuteRequestForTest(t, router, request)
			queries := counter.Measurement(counter.Queries())
			logCatalogQueries(t, "timespans", size, queries.Statements)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			assertCatalogData(t, response.Body.Bytes(), expected, "Time spans retrieved successfully")
			assertCatalogReadOnly(t, counter)
			require.Len(t, counter.Selects("schedule.timeframes"), 1, "one active-only tenant read")
			if size != 0 {
				require.Len(t, queries.Statements, previous, "statement count must not grow with N")
			}
			previous = len(queries.Statements)
			querybudgets.AssertQueryBudget(t, "api.activities.timespans", queries)
		})
	}
}

func assertCatalogData(t *testing.T, body []byte, expected []map[string]any, message string) {
	t.Helper()
	var envelope struct {
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	want, err := json.Marshal(expected)
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(envelope.Data), "exact DTOs and order, including empty arrays and foreign-tenant exclusion")
	require.Equal(t, message, envelope.Message)
}

func assertCatalogReadOnly(t *testing.T, counter *testpkg.QueryCounter) {
	t.Helper()
	require.Zero(t, counter.WriteRows())
	for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
		require.Empty(t, counter.Operation(operation), "list must issue no DML, including zero-row writes")
	}
}

func logCatalogQueries(t *testing.T, scenario string, size int, queries []string) {
	t.Helper()
	encoded, err := json.Marshal(queries)
	require.NoError(t, err)
	t.Logf("%s n=%d statements=%d queries_json=%s", scenario, size, len(queries), encoded)
}
