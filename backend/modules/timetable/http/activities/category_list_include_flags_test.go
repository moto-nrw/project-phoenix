//go:build testhooks

package httpadapter_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestActivityCategoriesListIncludeFlags(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	router, _, token := activitiesCatalogBudgetRouter(t, db)
	archived := time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC)
	// Insert in reverse name order. With no include flags all rows are hidden;
	// either flag alone must still load usage for the newly visible rows.
	categories := []testpkg.ActivityCategory{
		{TenantID: testpkg.Tenant(t), Name: "C system", IsSystem: true},
		{TenantID: testpkg.Tenant(t), Name: "B system archived", IsSystem: true, ArchivedAt: &archived},
		{TenantID: testpkg.Tenant(t), Name: "A archived", ArchivedAt: &archived},
	}
	for i := range categories {
		err := db.NewInsert().Model(&categories[i]).Returning("*").Scan(testpkg.Ctx(t))
		require.NoError(t, err)
		insertCatalogUsage(t, db, testpkg.Tenant(t), categories[i].ID, i+1)
	}
	foreign := testpkg.NewTenantScope(t, db)
	foreignCategory := testpkg.CreateTestActivityCategoryForTenant(t, db, foreign.TenantID, "Foreign visible")
	insertCatalogUsage(t, db, foreign.TenantID, foreignCategory.ID, 5)
	counter := testpkg.CaptureQueries(t, db)
	for _, includeSystem := range []bool{false, true} {
		for _, includeArchived := range []bool{false, true} {
			for _, withUsage := range []bool{false, true} {
				t.Run(fmt.Sprintf("system=%t/archived=%t/usage=%t", includeSystem, includeArchived, withUsage), func(t *testing.T) {
					want := []map[string]any{}
					for i := len(categories) - 1; i >= 0; i-- {
						category := categories[i]
						if (category.IsSystem && !includeSystem) || (category.ArchivedAt != nil && !includeArchived) {
							continue
						}
						row := map[string]any{"id": category.ID, "name": category.Name, "is_system": category.IsSystem, "created_at": category.CreatedAt, "updated_at": category.UpdatedAt}
						if category.ArchivedAt != nil {
							row["archived_at"] = category.ArchivedAt
						}
						if withUsage {
							row["usage_count"] = i + 1
						}
						want = append(want, row)
					}
					counter.Reset()
					path := fmt.Sprintf("/api/activities/categories?include_system=%t&include_archived=%t&with_usage=%t", includeSystem, includeArchived, withUsage)
					request := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(token))
					response := testutil.ExecuteRequestForTest(t, router, request)
					logCatalogQueries(t, path, len(want), counter.Measurement(counter.Queries()).Statements)
					require.Equal(t, http.StatusOK, response.Code, response.Body.String())
					assertCatalogData(t, response.Body.Bytes(), want, "Categories retrieved successfully")
					assertCatalogReadOnly(t, counter)
					require.Len(t, counter.Selects("activities.categories"), 1)
					if !withUsage {
						require.Empty(t, counter.Selects("activities.groups"))
						querybudgets.AssertQueryBudget(t, "api.activities.categories.without_usage", counter.Measurement(counter.Queries()))
					} else if len(want) == 0 {
						require.Empty(t, counter.Selects("activities.groups"))
						querybudgets.AssertQueryBudget(t, "api.activities.categories.empty_with_usage", counter.Measurement(counter.Queries()))
					} else {
						require.Len(t, counter.Selects("activities.groups"), 1)
						querybudgets.AssertQueryBudget(t, "api.activities.categories.with_usage", counter.Measurement(counter.Queries()))
					}
				})
			}
		}
	}
}
