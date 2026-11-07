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

type roomListBudgetWant struct {
	name           string
	groupName      string
	categoryName   string
	supervisorName string
}

// The existing root binding includes JWT, scope, permissions and the tenant
// transaction, but not the outer host's live-session middleware. Only the
// paginated list invokes Occupancy; available and by-category intentionally
// return simple room responses, even for occupied rooms.
func TestRoomListsQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	composedDB, router := httpadapter.RoomMetadataBudgetRouterForTest(t)
	require.Same(t, db, composedDB)
	account := testpkg.CreateTestAccount(t, db, "room-lists-admin")

	initialRooms, err := db.NewSelect().Model((*testpkg.Room)(nil)).
		Where("tenant_id = ?", testpkg.Tenant(t)).Count(context.Background())
	require.NoError(t, err)
	require.Zero(t, initialRooms, "this test owns a fresh tenant")

	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)
	foreignRoom := testpkg.CreateTestRoomForTenant(t, db, foreignTenant, "foreign-list-room")
	// Match every requested filter so by-category also exercises isolation.
	_, err = db.NewUpdate().Model(foreignRoom).
		Set("category = ?", "BudgetCategory").
		Where("id = ?", foreignRoom.ID).Where("tenant_id = ?", foreignTenant).
		Exec(context.Background())
	require.NoError(t, err)

	expected := make(map[int64]roomListBudgetWant)
	add := func(start, end int) {
		t.Helper()
		for i := start; i < end; i++ {
			name := fmt.Sprintf("list-room-%02d", i)
			room := testpkg.CreateTestRoom(t, db, name)
			_, err := db.NewUpdate().Model(room).
				Set("name = ?", name).Set("category = ?", "BudgetCategory").
				Where("id = ?", room.ID).Where("tenant_id = ?", testpkg.Tenant(t)).
				Exec(context.Background())
			require.NoError(t, err)

			group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("list-activity-%02d", i))
			active := testpkg.CreateTestActiveGroup(t, db, group.ID, room.ID)
			staff := testpkg.CreateTestStaff(t, db, "Budget", fmt.Sprintf("Supervisor%02d", i))
			testpkg.CreateTestGroupSupervisor(t, db, staff.ID, active.ID, "supervisor")
			student := testpkg.CreateTestStudent(t, db, "Budget", fmt.Sprintf("Visitor%02d", i), "3a")
			testpkg.CreateTestVisit(t, db, student.ID, active.ID, active.StartTime, nil)

			// The category fixture generates its own unique name. Its shared row
			// maps activities.categories; the room's category is a separate label.
			category := &testpkg.ActivityCategory{}
			require.NoError(t, db.NewSelect().Model(category).
				Where("category.id = ?", group.CategoryID).
				Where("category.tenant_id = ?", testpkg.Tenant(t)).
				Scan(context.Background()))
			require.NotNil(t, staff.Person)
			expected[room.ID] = roomListBudgetWant{
				name: name, groupName: group.Name, categoryName: category.Name,
				supervisorName: staff.Person.FirstName + " " + staff.Person.LastName,
			}
		}
	}

	routes := []struct {
		path      string
		occupancy bool
		message   string
	}{
		{"/?page=1&page_size=100", true, "Rooms retrieved successfully"},
		{"/available?capacity=1", false, "Available rooms retrieved successfully"},
		{"/by-category?category=BudgetCategory", false, "Rooms retrieved successfully"},
	}
	counter := testpkg.CaptureQueriesForContext(t, db)
	measure := func(n int) map[string][]string {
		t.Helper()
		require.Len(t, expected, n)
		statements := make(map[string][]string, len(routes))
		for _, route := range routes {
			counter.Reset()
			request := testutil.NewRequest(http.MethodGet, route.path, nil)
			request = request.WithContext(counter.Context(request.Context()))
			response := testutil.ExecuteWithAuth(t, router, request, testutil.AdminTestClaims(int(account.ID)))
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var body struct {
				Status     string                     `json:"status"`
				Message    string                     `json:"message"`
				Data       []httpadapter.RoomResponse `json:"data"`
				Pagination *struct {
					CurrentPage  int `json:"current_page"`
					PageSize     int `json:"page_size"`
					TotalRecords int `json:"total_records"`
				} `json:"pagination"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			require.Equal(t, "success", body.Status)
			require.Equal(t, route.message, body.Message)
			require.Len(t, body.Data, n)
			seen := make(map[int64]bool, n)
			for _, room := range body.Data {
				want, ok := expected[room.ID]
				require.True(t, ok, "unexpected or foreign room %d", room.ID)
				require.NotEqual(t, foreignRoom.ID, room.ID)
				require.False(t, seen[room.ID], "duplicate room")
				seen[room.ID] = true
				require.Equal(t, want.name, room.Name)
				require.Equal(t, "Test Building", room.Building)
				require.NotNil(t, room.Category)
				require.Equal(t, "BudgetCategory", *room.Category)
				require.NotNil(t, room.Capacity)
				require.Equal(t, 30, *room.Capacity)
				require.Nil(t, room.Floor)
				require.Nil(t, room.Color)
				require.False(t, room.IsSystem)
				require.False(t, room.IsOpenRoom)
				require.False(t, room.CreatedAt.IsZero())
				require.False(t, room.UpdatedAt.IsZero())
				if route.occupancy {
					require.True(t, room.IsOccupied)
					require.Equal(t, 1, room.StudentCount)
					require.NotNil(t, room.GroupName)
					require.Equal(t, want.groupName, *room.GroupName)
					require.NotNil(t, room.CategoryName)
					require.Equal(t, want.categoryName, *room.CategoryName)
					require.NotNil(t, room.SupervisorNames)
					require.Equal(t, want.supervisorName, *room.SupervisorNames)
				} else {
					require.False(t, room.IsOccupied)
					require.Zero(t, room.StudentCount)
					require.Nil(t, room.GroupName)
					require.Nil(t, room.CategoryName)
					require.Nil(t, room.SupervisorNames)
				}
			}
			require.Len(t, seen, len(expected), "every expected room must be returned")
			if route.occupancy {
				require.Len(t, counter.Selects("staff_school_memberships"), 1, "one membership set read")
				require.Len(t, counter.Selects("users.persons"), 1, "one person set read")
				require.NotNil(t, body.Pagination)
				require.Equal(t, 1, body.Pagination.CurrentPage)
				require.Equal(t, 100, body.Pagination.PageSize)
				require.Equal(t, n, body.Pagination.TotalRecords)
			} else {
				require.Nil(t, body.Pagination)
			}
			for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
				require.Empty(t, counter.Operation(operation), "room reads must not issue %s", operation)
			}
			require.Zero(t, counter.WriteRows())
			require.NotEmpty(t, counter.Selects("facilities.rooms"), "read persisted rooms")
			statements[route.path] = counter.Queries()
			t.Logf("room list %s N=%d SQL=%d", route.path, n, len(statements[route.path]))
		}
		return statements
	}

	add(0, 3)
	small := measure(3)
	withRoomEmploymentReadDenied(t, db, func() {
		counter.Reset()
		faultRequest := testutil.NewRequest(http.MethodGet, "/?page=1&page_size=100", nil)
		faultRequest = faultRequest.WithContext(counter.Context(faultRequest.Context()))
		faultResponse := testutil.ExecuteWithAuth(t, router, faultRequest, testutil.AdminTestClaims(int(account.ID)))
		assertRoomEmploymentReadFailure(t, counter, faultResponse)
	})
	add(3, 8)
	large := measure(8)
	for _, route := range routes {
		require.Len(t, large[route.path], len(small[route.path]), "%s grows with rooms", route.path)
	}
	querybudgets.AssertQueryBudget(t, "api.rooms.list", small["/?page=1&page_size=100"])
	querybudgets.AssertQueryBudget(t, "api.rooms.list", large["/?page=1&page_size=100"])
	querybudgets.AssertQueryBudget(t, "api.rooms.available", small["/available?capacity=1"])
	querybudgets.AssertQueryBudget(t, "api.rooms.available", large["/available?capacity=1"])
	querybudgets.AssertQueryBudget(t, "api.rooms.by_category", small["/by-category?category=BudgetCategory"])
	querybudgets.AssertQueryBudget(t, "api.rooms.by_category", large["/by-category?category=BudgetCategory"])

}
