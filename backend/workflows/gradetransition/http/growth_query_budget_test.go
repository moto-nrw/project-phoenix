package gradetransitionhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	gradehttp "github.com/moto-nrw/project-phoenix/workflows/gradetransition/http"
	"github.com/stretchr/testify/require"
)

// Each collection grows positively; the mounted production auth and tenant
// transaction chain is counted, not the fixture writes or typed root setup.
func TestGradeTransitionGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupGradeTransitionsModule(t)
	account := testpkg.CreateTestAccount(t, tc.db, "grade-growth")
	token := mintAdminToken(t, account.ID)
	router := testpkg.TenantRuntimeMiddleware(t, tc.db)(tc.resource.Router(api.ProtectedTenantRoutes))
	counter := testpkg.CaptureQueries(t, tc.db)
	transition := testpkg.CreateTestGradeTransition(t, tc.db, "2030-2031", account.ID)
	transitionIDs := []int64{transition.ID}
	var studentIDs []int64
	get := func(path string, target any) []string {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		counter.Reset()
		router.ServeHTTP(rec, req)
		queries := counter.Queries()
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		require.NoError(t, json.Unmarshal(envelope.Data, target))
		return queries
	}
	add := func(n int) {
		for range n {
			i := len(studentIDs)
			class := fmt.Sprintf("1%c", 'a'+i)
			next := fmt.Sprintf("2%c", 'a'+i)
			student := testpkg.CreateTestStudent(t, tc.db, fmt.Sprintf("Grade%d", i), "Growth", class)
			studentIDs = append(studentIDs, student.ID)
			testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, class, &next)
			testpkg.CreateTestGradeTransitionHistoryForTenant(t, tc.db, testpkg.Tenant(t), transition.ID, student.ID, fmt.Sprintf("Grade%d Growth", i), class, "promoted")
			if i > 0 {
				transitionIDs = append(transitionIDs, testpkg.CreateTestGradeTransition(t, tc.db, fmt.Sprintf("%d-%d", 2030+i, 2031+i), account.ID).ID)
			}
		}
	}
	run := func(n int) [][]string {
		var rows []gradehttp.TransitionResponse
		list := get("/?page_size=100", &rows)
		require.Len(t, rows, n)
		actual := make([]int64, 0, n)
		for _, row := range rows {
			actual = append(actual, row.ID)
			require.Equal(t, "draft", row.Status)
			require.Equal(t, account.ID, row.CreatedBy)
		}
		require.ElementsMatch(t, transitionIDs, actual)
		var classes []string
		classQ := get("/classes", &classes)
		require.Len(t, classes, n)
		for i, class := range classes {
			require.Equal(t, fmt.Sprintf("1%c", 'a'+i), class)
		}
		var detail gradehttp.TransitionResponse
		detailQ := get(fmt.Sprintf("/%d/", transition.ID), &detail)
		require.Equal(t, transition.ID, detail.ID)
		require.Equal(t, "2030-2031", detail.AcademicYear)
		require.Len(t, detail.Mappings, n)
		for i, mapping := range detail.Mappings {
			require.Equal(t, fmt.Sprintf("1%c", 'a'+i), mapping.FromClass)
			require.NotNil(t, mapping.ToClass)
			require.Equal(t, fmt.Sprintf("2%c", 'a'+i), *mapping.ToClass)
			require.Equal(t, "promoted", mapping.Action)
		}
		var history []gradehttp.HistoryResponse
		historyQ := get(fmt.Sprintf("/%d/history", transition.ID), &history)
		require.Len(t, history, n)
		actualStudents := make([]int64, 0, n)
		for _, row := range history {
			actualStudents = append(actualStudents, row.StudentID)
			require.Equal(t, transition.ID, row.TransitionID)
			require.Equal(t, "promoted", row.Action)
			require.NotEmpty(t, row.PersonName)
		}
		require.ElementsMatch(t, studentIDs, actualStudents)
		var preview gradehttp.PreviewResponse
		previewQ := get(fmt.Sprintf("/%d/preview", transition.ID), &preview)
		require.Equal(t, transition.ID, preview.TransitionID)
		require.Equal(t, n, preview.TotalStudents)
		require.Equal(t, n, preview.ToPromote)
		require.Zero(t, preview.ToGraduate)
		require.Empty(t, preview.UnmappedClasses)
		require.NotEmpty(t, preview.Fingerprint)
		require.Len(t, preview.ByMapping, n)
		for i, mapping := range preview.ByMapping {
			require.Equal(t, fmt.Sprintf("1%c", 'a'+i), mapping.FromClass)
			require.Equal(t, 1, mapping.StudentCount)
			require.Equal(t, "promote", mapping.Action)
		}
		return [][]string{list, classQ, detailQ, historyQ, previewQ}
	}
	add(3)
	small := run(3)
	add(5)
	large := run(8)
	for i := range large {
		t.Logf("grade route %d N3=%d N8=%d", i, len(small[i]), len(large[i]))
	}
	querybudgets.AssertQueryBudget(t, "api.grade_transitions.growth.list", large[0])
	querybudgets.AssertQueryBudget(t, "api.grade_transitions.growth.classes", large[1])
	querybudgets.AssertQueryBudget(t, "api.grade_transitions.growth.detail", large[2])
	querybudgets.AssertQueryBudget(t, "api.grade_transitions.growth.history", large[3])
	querybudgets.AssertQueryBudget(t, "api.grade_transitions.growth.preview", large[4])
}
