package careplanhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStudentCareWithdrawals_ChildScopedResolvedList pins the child's
// Änderungsprotokoll source for completed withdrawals (#5025): the request
// reviewers of the child read it with users:update or users:absence, and it
// never reaches beyond the one child, the resolved state or the reason.
func TestStudentCareWithdrawals_ChildScopedResolvedList(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRouteWithCareLifecycle(t, false)
	child := testpkg.CreateTestStudent(t, tc.db, "Api", "Abgemeldet", "4a")
	sibling := testpkg.CreateTestStudent(t, tc.db, "Api", "Nachbar", "4a")
	actor := testpkg.CreateTestAccount(t, tc.db, "student-care-withdrawals@example.com")
	firstGap := calendar.NewDate(2026, 9, 1)

	resolved := testpkg.CreateTestCareWithdrawalCompletion(t, tc.db, child.ID, actor.ID, firstGap)
	testpkg.ResolveTestCareWithdrawal(t, tc.db, resolved.ID)
	// A pending task of the same child and a resolved one of another child
	// stay out: the list is the child's completed withdrawals only.
	testpkg.CreateTestCareWithdrawalCompletion(t, tc.db, child.ID, actor.ID, firstGap.AddDays(7))
	foreignResolved := testpkg.CreateTestCareWithdrawalCompletion(t, tc.db, sibling.ID, actor.ID, firstGap)
	testpkg.ResolveTestCareWithdrawal(t, tc.db, foreignResolved.ID)

	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Betreuung", "Kraft")
	staff := testutil.TeacherTestClaims(int(staffAccount.ID))
	path := fmt.Sprintf("/%d/care-withdrawals", child.ID)

	for _, perms := range [][]string{{"users:absence"}, {"users:update"}, {"users:read", "users:absence"}} {
		rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil), staff, perms)
		require.Equal(t, http.StatusOK, rr.Code, "%v: %s", perms, rr.Body.String())
		var body struct {
			Data struct {
				Items []map[string]any `json:"items"`
				Total int              `json:"total"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		require.Len(t, body.Data.Items, 1, "%v", perms)
		assert.Equal(t, 1, body.Data.Total)
		item := body.Data.Items[0]
		assert.Equal(t, strconv.FormatInt(resolved.ID, 10), item["id"])
		assert.Equal(t, strconv.FormatInt(child.ID, 10), item["student_id"])
		assert.Equal(t, "resolved", item["state"])
		assert.Equal(t, "care_ended", item["outcome"])
		assert.NotContains(t, item, "reason", "the exit reason stays behind users:delete")
		assert.NotContains(t, item, "reason_note")
	}

	// Query parameters of the tenant list do not widen the child route.
	for _, query := range []string{
		"?state=pending",
		"?search=Nachbar",
		fmt.Sprintf("?student_id=%d", sibling.ID),
	} {
		rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodGet, path+query, nil), staff, []string{"users:absence"})
		require.Equal(t, http.StatusOK, rr.Code, "%s: %s", query, rr.Body.String())
		var body struct {
			Data struct {
				Items []struct {
					ID string `json:"id"`
				} `json:"items"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		require.Len(t, body.Data.Items, 1, query)
		assert.Equal(t, strconv.FormatInt(resolved.ID, 10), body.Data.Items[0].ID, query)
	}
}

func TestStudentCareWithdrawals_RejectsCallersWithoutChildAccess(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRouteWithCareLifecycle(t, false)
	child := testpkg.CreateTestStudent(t, tc.db, "Api", "Geschuetzt", "4b")
	actor := testpkg.CreateTestAccount(t, tc.db, "student-care-withdrawals-gate@example.com")
	completion := testpkg.CreateTestCareWithdrawalCompletion(t, tc.db, child.ID, actor.ID, calendar.NewDate(2026, 9, 1))
	testpkg.ResolveTestCareWithdrawal(t, tc.db, completion.ID)
	path := fmt.Sprintf("/%d/care-withdrawals", child.ID)

	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Nur", "Lesen")
	readOnly := authExec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil),
		testutil.TeacherTestClaims(int(staffAccount.ID)), []string{"users:read"})
	assert.Equal(t, http.StatusForbidden, readOnly.Code, "the route keeps the request log's gate")

	// An account that is no staff member of the school has no read access to
	// the child, whatever its permissions.
	noStaff := authExec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil),
		testutil.TeacherTestClaims(int(actor.ID)), []string{"users:read", "users:absence", "users:update"})
	assert.Equal(t, http.StatusForbidden, noStaff.Code, "Body: %s", noStaff.Body.String())
	assert.NotContains(t, noStaff.Body.String(), `"items"`)

	otherTenant, _ := testpkg.CreateTestTenant(t, tc.db)
	foreign := testpkg.CreateTestStudentForTenant(t, tc.db, otherTenant, "Fremd", "Kind", "4b")
	foreignResponse := authExec(t, tc,
		testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/%d/care-withdrawals", foreign.ID), nil),
		testutil.AdminTestClaims(int(actor.ID)), []string{"admin:*"})
	assert.Equal(t, http.StatusNotFound, foreignResponse.Code, "a child of another school does not exist here")
}

// TestListStudentCareWithdrawalsQueryBudget guards GET
// /students/{id}/care-withdrawals: one page costs the same statements and
// reads the same rows whether the child has one completed withdrawal or four.
func TestListStudentCareWithdrawalsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRouteWithCareLifecycle(t, false)
	child := testpkg.CreateTestStudent(t, tc.db, "Kind", "Abmeldeprotokoll", "QB3")
	actor := testpkg.CreateTestAccount(t, tc.db, "student-care-withdrawal-budget@example.com")
	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Budget", "Kraft")
	staff := testutil.TeacherTestClaims(int(staffAccount.ID))

	created := 0
	addResolved := func(n int) {
		for range n {
			completion := testpkg.CreateTestCareWithdrawalCompletion(t, tc.db, child.ID, actor.ID, calendar.NewDate(2026, 9, 1+created))
			testpkg.ResolveTestCareWithdrawal(t, tc.db, completion.ID)
			created++
		}
	}

	counter := testpkg.CaptureQueries(t, tc.db)
	type measurement struct {
		statements []string
		rows       int64
		total      int
		items      int
	}
	run := func() measurement {
		counter.Reset()
		path := fmt.Sprintf("/%d/care-withdrawals?page_size=1", child.ID)
		rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil), staff, []string{"users:absence"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
		var body struct {
			Data struct {
				Items []json.RawMessage `json:"items"`
				Total int               `json:"total"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		rows, _ := counter.Rows()
		return measurement{statements: counter.Queries(), rows: rows, total: body.Data.Total, items: len(body.Data.Items)}
	}

	addResolved(1)
	small := run()
	addResolved(3)
	large := run()

	t.Logf("1 withdrawal → %d statements/%d rows, 4 withdrawals → %d statements/%d rows",
		len(small.statements), small.rows, len(large.statements), large.rows)
	assert.Equal(t, 1, small.total)
	assert.Equal(t, 4, large.total, "total counts the child's completed withdrawals, not the page")
	assert.Equal(t, 1, large.items, "the page holds page_size tasks")
	assert.Len(t, large.statements, len(small.statements), "statement count must not grow with the child's withdrawals")
	assert.Equal(t, small.rows, large.rows, "rows read must not grow; only the requested page may be loaded")
	testpkg.AssertQueryBudget(t, "api.students.care_withdrawals.student_list", large.statements)
}
