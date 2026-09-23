package students_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestListCareWithdrawalsQueryBudget guards GET /care-withdrawals against
// reading the tenant's whole queue for one page (#3412). The statement count
// was flat even while the regression hydrated every withdrawal child, so the
// test also pins the rows the request reads: both must stay identical when
// the queue grows from one child to four, with and without a search that
// matches every child.
func TestListCareWithdrawalsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t)
	wireCareLifecycle(t, tc)
	repos := newStudentTestRepositories(tc.db)
	actor := testpkg.CreateTestAccount(t, tc.db, "care-withdrawal-budget@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))

	created := 0
	addWithdrawals := func(n int) {
		for range n {
			student := testpkg.CreateTestStudent(t, tc.db, fmt.Sprintf("Kind%d", created), "Abmeldebudget", "QB2")
			studentID := student.ID
			require.NoError(t, repos.CareWithdrawal.UpsertPending(testpkg.Ctx(t), &userModels.CareWithdrawalCompletion{
				StudentID: &studentID, FirstBookinglessDay: timezone.NewDate(2026, 9, 1+created),
				Trigger:               userModels.CareWithdrawalTriggerDirectSchool,
				WithdrawalConfirmedBy: &actor.ID, WithdrawalConfirmedRole: "admin", WithdrawalConfirmedAt: actor.CreatedAt,
			}))
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
	run := func(query string) measurement {
		counter.Reset()
		rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodGet, "/care-withdrawals"+query, nil), claims, []string{"users:delete"})
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

	const page, search = "?page_size=1", "?page_size=1&search=abmeldebudget"

	addWithdrawals(1)
	smallPage, smallSearch := run(page), run(search)

	addWithdrawals(3)
	largePage, largeSearch := run(page), run(search)

	for _, pair := range []struct {
		name         string
		small, large measurement
	}{{"page", smallPage, largePage}, {"search", smallSearch, largeSearch}} {
		t.Logf("%s: 1 child → %d statements/%d rows, 4 children → %d statements/%d rows",
			pair.name, len(pair.small.statements), pair.small.rows, len(pair.large.statements), pair.large.rows)
		assert.Equal(t, 1, pair.small.total, pair.name)
		assert.Equal(t, 4, pair.large.total, "%s: total counts the filtered queue, not the page", pair.name)
		assert.Equal(t, 1, pair.large.items, "%s: the page holds page_size tasks", pair.name)
		assert.Len(t, pair.large.statements, len(pair.small.statements),
			"%s: statement count must not grow with the queue", pair.name)
		assert.Equal(t, pair.small.rows, pair.large.rows,
			"%s: rows read must not grow with the queue; only the requested page may be loaded", pair.name)
	}
	testpkg.AssertQueryBudget(t, "api.students.care_withdrawals.list", largePage.statements)
}
