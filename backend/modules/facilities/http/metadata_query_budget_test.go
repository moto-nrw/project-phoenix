//go:build testhooks

package httpadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	httpadapter "github.com/moto-nrw/project-phoenix/modules/facilities/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// The existing root binding supplies the protected resource router. Its JWT,
// scope, permission and tenant transaction chain is included; outer serve-host
// live-session middleware is not part of this module-router fixture.
func TestRoomMetadataListsQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	composedDB, router := httpadapter.RoomMetadataBudgetRouterForTest(t)
	require.Same(t, db, composedDB)
	account := testpkg.CreateTestAccount(t, db, "metadata-admin")

	initialRooms, err := db.NewSelect().Model((*testpkg.Room)(nil)).Where("tenant_id = ?", testpkg.Tenant(t)).Count(context.Background())
	require.NoError(t, err)
	require.Zero(t, initialRooms, "this test owns a fresh tenant")
	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)
	testpkg.CreateTestRoomForTenant(t, db, foreignTenant, "metadata-foreign-room")
	values := []struct {
		building string
		category string
	}{
		{"Building A", "Category A"},
		{"Building B", "Category B"},
		{"Building A", ""},
		{"Building C", "Category C"},
		{"Building D", "Category D"},
		{"Building E", "Category E"},
		{"", ""},
		{"Building B", "Category B"},
	}
	addRooms := func(start, end int) {
		t.Helper()
		for i := start; i < end; i++ {
			room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("metadata-room-%02d", i))
			var category *string
			if values[i].category != "" {
				value := values[i].category
				category = &value
			}
			_, err := db.NewUpdate().Model(room).
				Set("building = ?", values[i].building).Set("category = ?", category).
				Where("id = ?", room.ID).Where("tenant_id = ?", testpkg.Tenant(t)).Exec(context.Background())
			require.NoError(t, err)
		}
	}

	counter := testpkg.CaptureQueriesForContext(t, db)
	measure := func(n int) map[string][]string {
		t.Helper()
		statements := make(map[string][]string, 2)
		for _, field := range []string{"buildings", "categories"} {
			prefix := "Building"
			if field == "categories" {
				prefix = "Category"
			}
			want := []string{prefix + " A", prefix + " B"}
			if n == 8 {
				want = append(want, prefix+" C", prefix+" D", prefix+" E")
			}
			counter.Reset()
			request := testutil.NewRequest(http.MethodGet, "/"+field, nil)
			request = request.WithContext(counter.Context(request.Context()))
			response := testutil.ExecuteWithAuth(t, router, request, testutil.AdminTestClaims(int(account.ID)))
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var body struct {
				Status  string              `json:"status"`
				Data    map[string][]string `json:"data"`
				Message string              `json:"message"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			require.Equal(t, "success", body.Status)
			require.Equal(t, prefix+" list retrieved successfully", body.Message)
			require.Equal(t, map[string][]string{field: want}, body.Data, "exact sorted distinct values, no blanks or foreign-tenant metadata")
			for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
				require.Empty(t, counter.Operation(operation), "metadata reads must not issue %s", operation)
			}
			require.Zero(t, counter.WriteRows())
			require.NotEmpty(t, counter.Selects("facilities.rooms"), "the successful response must read persisted rooms")
			statements[field] = counter.Queries()
			t.Logf("api.rooms.%s N=%d statements=%d", field, n, len(statements[field]))
		}
		return statements
	}

	addRooms(0, 3)
	small := measure(3)
	addRooms(3, 8)
	large := measure(8)
	for _, field := range []string{"buildings", "categories"} {
		require.Len(t, large[field], len(small[field]), "%s statements must not grow with rooms", field)

	}
	querybudgets.AssertQueryBudget(t, "api.rooms.buildings", small["buildings"])
	querybudgets.AssertQueryBudget(t, "api.rooms.buildings", large["buildings"])
	querybudgets.AssertQueryBudget(t, "api.rooms.categories", small["categories"])
	querybudgets.AssertQueryBudget(t, "api.rooms.categories", large["categories"])
}
