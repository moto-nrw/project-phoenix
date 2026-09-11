package students_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
		return strings.Contains(q, "users.staff") && strings.Contains(q, "person_id = ")
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
// test: one request through the production Router() (including the
// RequestIdentityCacheMiddleware mounted by ProtectedTenantGroup) resolves
// each identity-chain stage at most once, even though the aggregate calls
// GetMyGroups, GetSubstitutedGroupIDs, and ResolveStudentAccess internally.
//
// Before #2099 the same request resolved the person+staff pair ~4 times.
// The existing TestOGSGroupLive_QueryBudget bounds the total; this test pins
// the identity share specifically.
func TestOGSGroupLiveIdentityQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "IdentityBudget", "Leader")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "IdentityBudgetGroup")
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Substitution with an unassigned regular slot so both substitution
	// projections (GetMyGroups relations + GetSubstitutedGroupIDs) have a row.
	today := timezone.TodayDate()
	subGroup := testpkg.CreateTestEducationGroup(t, tc.db, "IdentityBudgetSubGroup")
	testpkg.CreateTestGroupSubstitution(t, tc.db, subGroup.ID, nil, teacher.StaffID,
		today.AddDays(-1), today.AddDays(3))

	for i := range 3 {
		student := testpkg.CreateTestStudent(t, tc.db, "IdentityBudget", fmt.Sprintf("Kind%d", i), "IB1")
		testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	}

	counter := testpkg.CaptureQueries(t, tc.db)

	req := testutil.NewRequest("GET", fmt.Sprintf("/ogs-group-live?group_id=%d", group.ID), nil)
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), ogsLivePerms)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	stage := func(name string) []string {
		queries := counter.Matching(func(q string) bool {
			return strings.HasPrefix(strings.TrimSpace(q), "select") && identityStageMatchers[name](q)
		})
		t.Logf("identity stage %q: %d queries", name, len(queries))
		return queries
	}

	// Each stage must be resolved exactly once per request. Two distinct
	// substitution projections exist (GetMyGroups relations,
	// GetSubstitutedGroupIDs id-set); each memoizes its own result, so 2 is
	// the per-request floor.
	testpkg.AssertQueryBudget(t, "api.students.ogs_group_live.identity.person", stage("person"))
	testpkg.AssertQueryBudget(t, "api.students.ogs_group_live.identity.staff", stage("staff"))
	testpkg.AssertQueryBudget(t, "api.students.ogs_group_live.identity.teacher", stage("teacher"))
	testpkg.AssertQueryBudget(t, "api.students.ogs_group_live.identity.substitutions", stage("substitutions"))
}
