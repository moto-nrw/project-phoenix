package grouplivehttp_test

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// identityStageMatchers buckets the SELECTs that resolve the identity chain
// (Account → Person → Staff → Teacher → substitutions) by a predicate over the
// lowercased SQL. The markers pair the table with the lookup column of the
// identity resolution so that business queries which merely JOIN the same
// tables (e.g. substitute names on the transfers projection) do not pollute
// the buckets.
// The substitution stage accepts both SQL shapes: the hand-written finder
// emits `.substitute_staff_id = ?`, the Filter-based one
// `"substitute_staff_id" = ?`; substitution reads filtered by group (the
// transfers projection) don't match either.
var identityStageMatchers = map[string]func(string) bool{
	"person": func(q string) bool {
		return strings.Contains(q, "users.persons") && strings.Contains(q, "account_id = ")
	},
	"staff": func(q string) bool {
		return strings.Contains(q, "users.staff_school_memberships") && strings.Contains(q, "person_id = ")
	},
	"teacher": func(q string) bool {
		return strings.Contains(q, "users.teachers") && strings.Contains(q, "staff_id = ")
	},
	"substitutions": func(q string) bool {
		hasTable := strings.Contains(q, "education.group_substitution") ||
			strings.Contains(q, `"education"."group_substitution"`)
		return hasTable &&
			(strings.Contains(q, "substitute_staff_id = ") || strings.Contains(q, `substitute_staff_id" = `))
	},
}

// TestOGSGroupLiveIdentityQueryBudget is the endpoint-level #2099 acceptance
// test: cold and warm requests through the native session and tenant middleware
// (including live-family validation and RequestIdentityCacheMiddleware) resolves
// each identity-chain stage at most once, even though the aggregate calls
// GetMyGroups, GetSubstitutedGroupIDs, and ResolveStudentAccess internally.
//
// Before #2099 the same request resolved the person+staff pair ~4 times.
// The existing TestOGSGroupLive_QueryBudget bounds the total; this test pins
// the identity share specifically.
//
// The request runs on fixedCalendarClock, a school Monday: the live view
// reads the day's pickup plan only on a care weekday, so a wall clock would
// make the count depend on the day the suite runs (#6266).
func TestOGSGroupLiveIdentityQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t, fixedCalendarClock)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "IdentityBudget", "Leader")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "IdentityBudgetGroup")
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Substitution with an unassigned regular slot so both substitution
	// projections (GetMyGroups relations + GetSubstitutedGroupIDs) have a row.
	// The identity chain still reads substitutions on the wall-clock day
	// (schoolstructure has no clock seam), the live view on the fixed day, so
	// the fixed range covers both.
	today := calendar.DateFromTime(fixedCalendarClock())
	subGroup := testpkg.CreateTestEducationGroup(t, tc.db, "IdentityBudgetSubGroup")
	testpkg.CreateTestGroupSubstitution(t, tc.db, subGroup.ID, nil, teacher.StaffID,
		today.AddDays(-1), calendar.NewDate(2099, time.December, 31))
	auth, err := api.NewAuthTestModule(testpkg.NewDatabase(t, tc.db), testpkg.TenantRuntime(t, tc.db))
	require.NoError(t, err)
	verifier, err := testpkg.ConfiguredTokenAuth()
	require.NoError(t, err)
	claims := testutil.TeacherTestClaims(int(account.ID))
	claims.Permissions = ogsLivePerms
	token := testpkg.MintLiveSessionJWT(t, tc.db, claims)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, tc.db), api.RequestSettingsCacheMiddleware, api.RequestIdentityCacheMiddleware)
	router.Use(verifier.Verifier(), api.RequireLiveAccessSession(auth.Auth))
	api.ProtectedTenantRoutes(router, tc.resource.Register)

	var studentIDs []string
	addStudents := func(n int) {
		for range n {
			student := testpkg.CreateTestStudent(t, tc.db, "IdentityBudget", fmt.Sprintf("Kind%d", len(studentIDs)), "IB1")
			testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
			studentIDs = append(studentIDs, strconv.FormatInt(student.ID, 10))
		}
	}

	counter := testpkg.CaptureQueries(t, tc.db)
	measure := func() map[string]int {
		counter.Reset()
		req := testutil.NewRequest("GET", fmt.Sprintf("/ogs-group-live?group_id=%d", group.ID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := testutil.ExecuteRequest(testpkg.MountedQueryBudgetRouter("/api/students", router), req)
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
		data := decodeOGSLive(t, rr.Body.Bytes()).Data
		require.Len(t, data.Students, len(studentIDs))
		actualIDs := make([]string, 0, len(data.Students))
		for _, student := range data.Students {
			id, ok := student["id"].(string)
			require.True(t, ok, "student IDs remain JSON strings")
			actualIDs = append(actualIDs, id)
		}
		require.ElementsMatch(t, studentIDs, actualIDs, "the budget covers all real group members")
		stage := func(name string) testpkg.QueryMeasurement {
			queries := counter.Matching(func(q string) bool {
				return strings.HasPrefix(strings.TrimSpace(q), "select") && identityStageMatchers[name](q)
			})
			t.Logf("N%d identity stage %q: %d queries", len(studentIDs), name, len(queries))
			return counter.Measurement(queries)
		}

		person, staff, teacher, substitutions := stage("person"), stage("staff"), stage("teacher"), stage("substitutions")
		querybudgets.AssertQueryBudget(t, "api.students.ogs_group_live.identity.person", person)
		querybudgets.AssertQueryBudget(t, "api.students.ogs_group_live.identity.staff", staff)
		querybudgets.AssertQueryBudget(t, "api.students.ogs_group_live.identity.teacher", teacher)
		querybudgets.AssertQueryBudget(t, "api.students.ogs_group_live.identity.substitutions", substitutions)
		t.Logf("live middleware N%d total=%d", len(studentIDs), counter.Total())
		querybudgets.AssertQueryBudget(t, "api.students.ogs_group_live.identity.live_middleware", counter.Measurement(counter.Queries()))
		return map[string]int{"person": len(person.Statements), "staff": len(staff.Statements), "teacher": len(teacher.Statements), "substitutions": len(substitutions.Statements)}
	}

	addStudents(3)
	small := measure()
	require.Equal(t, small, measure(), "a warm request still has its own identity memo")
	addStudents(5)
	require.Equal(t, small, measure(), "identity buckets must not grow with N3/N8 children")
	require.Equal(t, small, measure(), "warm N8 has the same identity budget")
}
