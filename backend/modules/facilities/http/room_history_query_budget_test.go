//go:build testhooks

package httpadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	httpadapter "github.com/moto-nrw/project-phoenix/modules/facilities/http"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

type roomHistoryBudgetWant struct {
	startedAt      time.Time
	endedAt        *time.Time
	activityName   string
	supervisorName string
	studentCount   int
}

// Measure the real protected Resource and its root-bound owner projections.
// The cardinality axis is sessions in ONE room, not the number of rooms.
// Outer serve-host live-session verification is outside this module router.
func TestRoomHistoryQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	composedDB, router := httpadapter.RoomHistoryBudgetRouterForTest(t)
	require.Same(t, db, composedDB)
	account := testpkg.CreateTestAccount(t, db, "room-history-admin")
	claims := testutil.AdminTestClaims(int(account.ID))

	// Calendar day and DATE fields use calendar.Date. Only session/visit
	// instants and RFC3339 query bounds use time.Time. Explicit bounds avoid
	// depending on the wall clock when the handler supplies its defaults.
	day := calendar.NewDate(2026, 10, 25) // Berlin DST fallback: explicit 25-hour query day.
	windowStart, windowEnd := day.BerlinMidnight(), day.EndOfDay()
	anchor := windowStart.Add(6 * time.Hour)
	room := testpkg.CreateTestRoom(t, db, "history-budget-room")
	expected := make(map[int64]roomHistoryBudgetWant)
	createdSessionIDs := make([]int64, 0, 8)
	add := func(start, end int) {
		t.Helper()
		for i := start; i < end; i++ {
			activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("History Activity %02d", i))
			session := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
			startedAt := anchor.Add(time.Duration(i) * 30 * time.Minute)
			var endedAt *time.Time
			if i%2 == 1 {
				ended := startedAt.Add(20 * time.Minute)
				endedAt = &ended
			}
			_, err := db.NewUpdate().Model(session).
				Set("start_time = ?", startedAt).Set("end_time = ?", endedAt).
				Set("last_activity = ?", startedAt).
				Where("id = ?", session.ID).Where("tenant_id = ?", testpkg.Tenant(t)).
				Exec(context.Background())
			require.NoError(t, err)

			staff := testpkg.CreateTestStaff(t, db, "History", fmt.Sprintf("Supervisor%02d", i))
			supervision := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, session.ID, "supervisor")
			var supervisionEnd *calendar.Date
			if endedAt != nil {
				supervisionEnd = &day
			}
			_, err = db.NewUpdate().Model(supervision).
				Set("start_date = ?", day).Set("end_date = ?", supervisionEnd).
				Where("id = ?", supervision.ID).Where("tenant_id = ?", testpkg.Tenant(t)).
				Exec(context.Background())
			require.NoError(t, err)
			require.NotNil(t, staff.Person)
			// A closed historical session must still resolve an offboarded member.
			// Keep the Person live; only School Membership is soft-deleted.
			if i == 1 {
				require.NotNil(t, endedAt)
				retiredAt := endedAt.Add(time.Minute)
				_, err = db.NewUpdate().Model(staff).ModelTableExpr("users.staff_school_memberships AS staff").Set("deleted_at = ?", retiredAt).
					Where("staff.id = ?", staff.ID).Where("staff.tenant_id = ?", testpkg.Tenant(t)).Exec(context.Background())
				require.NoError(t, err)
			}

			count := 1 + i%3
			for j := range count {
				student := testpkg.CreateTestStudent(t, db, "History", fmt.Sprintf("Visitor%02d-%02d", i, j), "3a")
				testpkg.CreateTestVisit(t, db, student.ID, session.ID, startedAt.Add(time.Minute), endedAt)
			}
			expected[session.ID] = roomHistoryBudgetWant{
				startedAt: startedAt, endedAt: endedAt, activityName: activity.Name,
				supervisorName: staff.Person.FirstName + " " + staff.Person.LastName,
				studentCount:   count,
			}
			createdSessionIDs = append(createdSessionIDs, session.ID)
		}
	}

	// A real foreign school has a matching in-window session, supervisor and
	// visit. Separate room IDs are required by the composite tenant FKs.
	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)
	foreignRoom := testpkg.CreateTestRoomForTenant(t, db, foreignTenant, "foreign-history-room")
	foreignActivity := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenant, "Foreign History Activity")
	foreignSession := testpkg.CreateTestActiveGroupWithIDsForTenant(t, db, foreignTenant, foreignActivity.ID, foreignRoom.ID)
	foreignEnd := anchor.Add(20 * time.Minute)
	_, err := db.NewUpdate().Model(foreignSession).
		Set("start_time = ?", anchor).Set("end_time = ?", foreignEnd).Set("last_activity = ?", anchor).
		Where("id = ?", foreignSession.ID).Where("tenant_id = ?", foreignTenant).
		Exec(context.Background())
	require.NoError(t, err)
	foreignStaff := testpkg.CreateTestStaffForTenant(t, db, foreignTenant, "Foreign", "HistorySupervisor")
	foreignSupervision := &testpkg.GroupSupervisorRow{
		TenantID: foreignTenant, StaffID: foreignStaff.ID, GroupID: foreignSession.ID,
		Role: "supervisor", StartDate: day, EndDate: &day,
	}
	err = db.NewInsert().Model(foreignSupervision).Scan(context.Background())
	require.NoError(t, err)
	foreignStudent := testpkg.CreateTestStudentForTenant(t, db, foreignTenant, "Foreign", "HistoryVisitor", "3a")
	testpkg.CreateTestVisitForTenant(t, db, foreignTenant, foreignStudent.ID, foreignSession.ID, anchor.Add(time.Minute), &foreignEnd)

	counter := testpkg.CaptureQueriesForContext(t, db)
	measure := func(n int) []string {
		t.Helper()
		require.Len(t, expected, n)
		require.Len(t, createdSessionIDs, n)
		request := testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/history", room.ID), nil)
		query := request.URL.Query()
		query.Set("start", windowStart.Format(time.RFC3339))
		query.Set("end", windowEnd.Format(time.RFC3339))
		request.URL.RawQuery = query.Encode()
		counter.Reset()
		request = request.WithContext(counter.Context(request.Context()))
		response := testutil.ExecuteWithAuth(t, router, request, claims)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var body struct {
			Status  string                         `json:"status"`
			Message string                         `json:"message"`
			Data    []httpadapter.RoomSessionEntry `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		require.Equal(t, "success", body.Status)
		require.Equal(t, "Room history retrieved successfully", body.Message)
		require.Len(t, body.Data, n)
		seen := make(map[int64]bool, n)
		for index, entry := range body.Data {
			want, ok := expected[entry.SessionID]
			require.True(t, ok, "unexpected or foreign session %d", entry.SessionID)
			require.NotEqual(t, foreignSession.ID, entry.SessionID)
			require.False(t, seen[entry.SessionID], "duplicate session")
			seen[entry.SessionID] = true
			// Starts are distinct, so the SQL's start_time DESC ordering is exact.
			require.Equal(t, createdSessionIDs[n-1-index], entry.SessionID)
			require.True(t, entry.StartedAt.Equal(want.startedAt))
			require.Equal(t, want.activityName, entry.ActivityName)
			require.Equal(t, want.supervisorName, entry.SupervisorName)
			require.Equal(t, want.studentCount, entry.StudentCount)
			if want.endedAt == nil {
				require.Nil(t, entry.EndedAt)
				require.Nil(t, entry.DurationMinutes)
			} else {
				require.NotNil(t, entry.EndedAt)
				require.True(t, entry.EndedAt.Equal(*want.endedAt))
				require.NotNil(t, entry.DurationMinutes)
				require.Equal(t, 20, *entry.DurationMinutes)
			}
		}
		require.Len(t, seen, len(expected))
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
			require.Empty(t, counter.Operation(operation), "history GET must not issue %s", operation)
		}
		require.Zero(t, counter.WriteRows())
		require.NotEmpty(t, counter.Selects("facilities.rooms"), "real room visibility check")
		require.NotEmpty(t, counter.Selects("active.groups"), "real session history")
		require.Len(t, counter.Selects("staff_school_memberships"), 1, "one membership set read")
		require.Len(t, counter.Selects("users.persons"), 1, "one person set read")
		require.Empty(t, counter.Selects("staff_employment_profiles"), "names do not require employment hydration")
		queries := counter.Queries()
		t.Logf("room history N=%d sessions in one room: statements=%d\n%s", n, len(queries), strings.Join(queries, "\n"))
		return queries
	}

	add(0, 3)
	small := measure(3)
	add(3, 8)
	large := measure(8)
	require.Len(t, large, len(small), "history statements must not grow with sessions or distinct supervisors")

	// Unlike merely placing rows in another room, this request exercises the
	// real room visibility gate with a foreign ID and the local caller. The
	// current history adapter renders the failed owner lookup as 500, not 404.
	foreignRequest := testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/history", foreignRoom.ID), nil)
	foreignQuery := foreignRequest.URL.Query()
	foreignQuery.Set("start", windowStart.Format(time.RFC3339))
	foreignQuery.Set("end", windowEnd.Format(time.RFC3339))
	foreignRequest.URL.RawQuery = foreignQuery.Encode()
	counter.Reset()
	foreignRequest = foreignRequest.WithContext(counter.Context(foreignRequest.Context()))
	foreignResponse := testutil.ExecuteWithAuth(t, router, foreignRequest, claims)
	require.Equal(t, http.StatusInternalServerError, foreignResponse.Code, foreignResponse.Body.String())
	require.NotContains(t, foreignResponse.Body.String(), foreignActivity.Name)
	require.NotContains(t, foreignResponse.Body.String(), "Foreign HistorySupervisor")
	for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
		require.Empty(t, counter.Operation(operation))
	}
	require.Zero(t, counter.WriteRows())
	querybudgets.AssertQueryBudget(t, "api.rooms.history", small)
	querybudgets.AssertQueryBudget(t, "api.rooms.history", large)
}
