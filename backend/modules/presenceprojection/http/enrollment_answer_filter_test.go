package presenceprojectionhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registrationAnswers registers one family whose children answered the
// school's own swimming question, each child linked to a student.
func registrationAnswers(t *testing.T, tc *testContext, answers map[int64]bool) {
	t.Helper()
	account := testpkg.CreateTestAccount(t, tc.db, "answers")
	fields := `[{"key":"seepferdchen","label":"Seepferdchen vorhanden","type":"boolean","applies_to_child":true,"sort_order":0}]`
	var schemaID int64
	require.NoError(t, tc.db.NewRaw(`INSERT INTO enrollment.form_schemas (tenant_id, name, version, fields, is_active, created_by)
		VALUES (?, ?, 1, ?::jsonb, true, ?) RETURNING id`, testpkg.Tenant(t), fmt.Sprintf("Anmeldung-%d", testpkg.UniqueSuffix()), fields, account.ID).
		Scan(testpkg.Ctx(t), &schemaID))
	phase := testpkg.CreateTestEnrollmentPhaseAround(t, tc.db, studentsTestToday)
	request := &enrollment.Request{
		PhaseID: phase.ID, SchemaID: &schemaID, GuardianFirstName: "Antwort", GuardianLastName: "Eltern",
		GuardianEmail: fmt.Sprintf("answers-%d@example.test", testpkg.UniqueSuffix()),
		StatusToken:   fmt.Sprintf("answers-%d", testpkg.UniqueSuffix()),
	}
	request.TenantID = testpkg.Tenant(t)
	owner := api.NewEnrollmentBookingFixture()
	ownerCtx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), tc.db)
	require.NoError(t, testpkg.TenantCommand1(t, ownerCtx, tc.db, owner.InsertRequest, request))
	for studentID, answer := range answers {
		child := &enrollment.RequestChild{
			RequestID: request.ID, FirstName: "Antwort", LastName: fmt.Sprintf("Kind%d", studentID),
			DateOfBirth: studentsTestToday.AddDays(-2500), Status: enrollment.ChildStatusApproved, CreatedStudentID: &studentID,
		}
		child.CustomData.Set("seepferdchen", answer)
		child.TenantID = testpkg.Tenant(t)
		require.NoError(t, testpkg.TenantCommand1(t, ownerCtx, tc.db, owner.InsertChild, child))
	}
}

func listByAnswer(t *testing.T, tc *testContext, answer string, perms ...string) (int, []int64) {
	t.Helper()
	path := "/?page_size=100&enrollment_answer=" + url.QueryEscape(answer)
	rr := authExec(t, tc, testutil.NewRequest("GET", path, nil), testutil.AdminTestClaims(1), perms)
	if rr.Code != http.StatusOK {
		return rr.Code, nil
	}
	var resp struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	ids := make([]int64, 0, len(resp.Data))
	for _, row := range resp.Data {
		ids = append(ids, row.ID)
	}
	return rr.Code, ids
}

func TestListStudentsFiltersByARegistrationAnswer(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	swimmer := testpkg.CreateTestStudent(t, tc.db, "Svea", "Schwimmer", "2a").ID
	learner := testpkg.CreateTestStudent(t, tc.db, "Lio", "Lerner", "2a").ID
	unanswered := testpkg.CreateTestStudent(t, tc.db, "Ute", "Ohne", "2a").ID
	registrationAnswers(t, tc, map[int64]bool{swimmer: true, learner: false})

	code, ids := listByAnswer(t, tc, "seepferdchen=true", "users:read")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, []int64{swimmer}, ids)
	_, ids = listByAnswer(t, tc, "seepferdchen=false", "users:read")
	assert.Equal(t, []int64{learner}, ids)
	assert.NotContains(t, ids, unanswered)
	_, ids = listByAnswer(t, tc, "unbekannt=true", "users:read")
	assert.Empty(t, ids, "an unknown question finds nobody instead of everybody")

	rr := authExec(t, tc, testutil.NewRequest("GET", "/enrollment-answer-filters", nil), testutil.AdminTestClaims(1), []string{"users:read"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var options struct {
		Data []struct {
			Key     string `json:"key"`
			Label   string `json:"label"`
			Options []struct {
				Value string `json:"value"`
				Label string `json:"label"`
				Count int    `json:"count"`
			} `json:"options"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &options))
	require.Len(t, options.Data, 1)
	assert.Equal(t, "Seepferdchen vorhanden", options.Data[0].Label)
	require.Len(t, options.Data[0].Options, 2)
	assert.Equal(t, 1, options.Data[0].Options[0].Count)
}

func TestListStudentsAnswerFilterNeedsUsersRead(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	code, _ := listByAnswer(t, tc, "seepferdchen=true", "students:read")
	assert.Equal(t, http.StatusForbidden, code, "a filtered count would reveal the answers")
	rr := authExec(t, tc, testutil.NewRequest("GET", "/enrollment-answer-filters", nil), testutil.AdminTestClaims(1), []string{"students:read"})
	assert.Equal(t, http.StatusForbidden, rr.Code)
	for _, malformed := range []string{"seepferdchen", "=true", "seepferdchen="} {
		code, _ := listByAnswer(t, tc, malformed, "users:read")
		assert.Equal(t, http.StatusBadRequest, code, malformed)
	}
}

// TestEnrollmentAnswerFilterOptionsQueryBudget keeps the filter's question
// list one read of the answers and one of their form versions, whatever the
// number of children who answered.
func TestEnrollmentAnswerFilterOptionsQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t, fixedCalendarClock)
	answered := func(n int) {
		answers := map[int64]bool{}
		for i := range n {
			answers[testpkg.CreateTestStudent(t, tc.db, "Budget", "Antwort", "1a").ID] = i%2 == 0
		}
		registrationAnswers(t, tc, answers)
	}
	counter := testpkg.CaptureQueries(t, tc.db)
	run := func() int {
		counter.Reset()
		rr := authExec(t, tc, testutil.NewRequest("GET", "/enrollment-answer-filters", nil), testutil.AdminTestClaims(1), []string{"users:read"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		return counter.Total()
	}
	answered(3)
	small := run()
	answered(7)
	large := run()
	assert.Equal(t, small, large, "query count must not grow with the number of answers")
	querybudgets.AssertQueryBudget(t, "api.students.enrollment_answer_filters.list", counter.Measurement(counter.Queries()))
}
