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
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
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

// answerReader is a staff member of the school with users:read: the access
// the answers on the child's page need.
func answerReader(t *testing.T, tc *testContext) sessiontoken.AppClaims {
	t.Helper()
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Antwort", "Leserin")
	return testutil.TeacherTestClaims(int(account.ID))
}

func listByAnswer(t *testing.T, tc *testContext, claims sessiontoken.AppClaims, answer string, perms ...string) (int, []int64) {
	t.Helper()
	path := "/?page_size=100&enrollment_answer=" + url.QueryEscape(answer)
	rr := authExec(t, tc, testutil.NewRequest("GET", path, nil), claims, perms)
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
	reader := answerReader(t, tc)

	code, ids := listByAnswer(t, tc, reader, "seepferdchen=true", "users:read")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, []int64{swimmer}, ids)
	_, ids = listByAnswer(t, tc, reader, "seepferdchen=false", "users:read")
	assert.Equal(t, []int64{learner}, ids)
	assert.NotContains(t, ids, unanswered)
	_, ids = listByAnswer(t, tc, reader, "unbekannt=true", "users:read")
	assert.Empty(t, ids, "an unknown question finds nobody instead of everybody")

	rr := authExec(t, tc, testutil.NewRequest("GET", "/enrollment-answer-filters", nil), reader, []string{"users:read"})
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
	reader := answerReader(t, tc)
	code, _ := listByAnswer(t, tc, reader, "seepferdchen=true", "students:read")
	assert.Equal(t, http.StatusForbidden, code, "a filtered count would reveal the answers")
	rr := authExec(t, tc, testutil.NewRequest("GET", "/enrollment-answer-filters", nil), reader, []string{"students:read"})
	assert.Equal(t, http.StatusForbidden, rr.Code)
	for _, malformed := range []string{"seepferdchen", "=true", "seepferdchen="} {
		code, _ := listByAnswer(t, tc, reader, malformed, "users:read")
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
	reader := answerReader(t, tc)
	counter := testpkg.CaptureQueries(t, tc.db)
	run := func() int {
		counter.Reset()
		rr := authExec(t, tc, testutil.NewRequest("GET", "/enrollment-answer-filters", nil), reader, []string{"users:read"})
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

func answerFilterCounts(t *testing.T, tc *testContext, reader sessiontoken.AppClaims) map[string]int {
	t.Helper()
	rr := authExec(t, tc, testutil.NewRequest("GET", "/enrollment-answer-filters", nil), reader, []string{"users:read"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var options struct {
		Data []struct {
			Key     string `json:"key"`
			Options []struct {
				Value string `json:"value"`
				Count int    `json:"count"`
			} `json:"options"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &options))
	counts := map[string]int{}
	for _, filter := range options.Data {
		for _, option := range filter.Options {
			counts[filter.Key+"="+option.Value] = option.Count
		}
	}
	return counts
}

// TestEnrollmentAnswerFilterCountsWhatTheListShows keeps the number next to an
// answer equal to the children the list shows for it: a child who left the
// school keeps its registration, but neither the list nor the count shows it.
func TestEnrollmentAnswerFilterCountsWhatTheListShows(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	stays := testpkg.CreateTestStudent(t, tc.db, "Bleibt", "Schwimmer", "3a").ID
	left := testpkg.CreateTestStudent(t, tc.db, "Ging", "Schwimmer", "3a").ID
	learner := testpkg.CreateTestStudent(t, tc.db, "Lernt", "Noch", "3a").ID
	registrationAnswers(t, tc, map[int64]bool{stays: true, left: true, learner: false})
	_, err := tc.db.ExecContext(testpkg.Ctx(t), `UPDATE users.student_school_memberships SET status = 'alumnus' WHERE student_profile_id = ?`, left)
	require.NoError(t, err)

	reader := answerReader(t, tc)
	_, listed := listByAnswer(t, tc, reader, "seepferdchen=true", "users:read")
	require.Equal(t, []int64{stays}, listed)
	counts := answerFilterCounts(t, tc, reader)
	assert.Equal(t, len(listed), counts["seepferdchen=true"], "the count is what the list shows")
	assert.Equal(t, 1, counts["seepferdchen=false"])
}

// TestEnrollmentAnswerFilterNeedsTheChildPageAccess gives the filter, its
// options and the filtered export the gate of the answers on the child's
// page: users:read and a staff record of the school, or an administrator.
func TestEnrollmentAnswerFilterNeedsTheChildPageAccess(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	guest := testpkg.CreateTestAccount(t, tc.db, "answers-guest@example.com")
	claims := testutil.TeacherTestClaims(int(guest.ID))
	read := []string{"users:read"}

	list := authExec(t, tc, testutil.NewRequest("GET", "/?enrollment_answer=seepferdchen%3Dtrue", nil), claims, read)
	assert.Equal(t, http.StatusForbidden, list.Code, list.Body.String())
	options := authExec(t, tc, testutil.NewRequest("GET", "/enrollment-answer-filters", nil), claims, read)
	assert.Equal(t, http.StatusForbidden, options.Code, options.Body.String())
	export := authExec(t, tc, birthdayExportRequest(t, `{"format":"xlsx","preset":"birthday_list","filters":{"months":["09"],"enrollment_answer":"seepferdchen=true"}}`), claims, read)
	assert.Equal(t, http.StatusForbidden, export.Code)

	// Anchor: the same account reads the list and exports without the filter.
	plain := authExec(t, tc, testutil.NewRequest("GET", "/", nil), claims, read)
	assert.Equal(t, http.StatusOK, plain.Code, plain.Body.String())
	unfiltered := authExec(t, tc, birthdayExportRequest(t, `{"format":"xlsx","preset":"birthday_list","filters":{"months":["09"]}}`), claims, read)
	assert.Equal(t, http.StatusOK, unfiltered.Code)
}
