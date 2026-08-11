package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// listMultiFilterStudentIDs runs a list request through the production router and returns
// the ids it answered with, plus the reported total. Both matter for #2218: a
// filter that only narrows the rendered page while the count keeps naming the
// unfiltered total would put a wrong number in front of the group leader.
func listMultiFilterStudentIDs(t *testing.T, tc *testContext, query string) ([]int64, int) {
	t.Helper()

	req := testutil.NewRequest("GET", "/?"+query, nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

	var resp struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
		Pagination struct {
			TotalRecords int `json:"total_records"`
		} `json:"pagination"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	ids := make([]int64, 0, len(resp.Data))
	for _, student := range resp.Data {
		ids = append(ids, student.ID)
	}
	return ids, resp.Pagination.TotalRecords
}

// assignStudentGroup puts a child into an educational group. No fixture does
// this today, and the multi-group filter is exactly about which group a child
// belongs to.
func assignStudentGroup(t *testing.T, tc *testContext, studentID, groupID int64) {
	t.Helper()

	_, err := tc.db.NewUpdate().
		Model((*usersModel.Student)(nil)).
		ModelTableExpr("users.students").
		Set("group_id = ?", groupID).
		Where("id = ?", studentID).
		Exec(context.Background())
	require.NoError(t, err)
}

// Two groups supervised together need one list covering both their classes
// (#2218). The filters therefore accept several values, and the result must be
// the union — not the first value, and not everything.
func TestListStudents_MultiValueClassGroupAndGradeFilters(t *testing.T) {
	tc := setupTestContext(t)

	// The suffix keeps the class names unique in the shared test database while
	// the leading digit still carries the grade schoolclass.GradePrefix reads.
	suffix := time.Now().UnixNano()
	classThird := fmt.Sprintf("3a-%d", suffix)
	classFourth := fmt.Sprintf("4b-%d", suffix)
	classSecond := fmt.Sprintf("2c-%d", suffix)

	third := testpkg.CreateTestStudent(t, tc.db, "Multi", "Third", classThird)
	fourth := testpkg.CreateTestStudent(t, tc.db, "Multi", "Fourth", classFourth)
	second := testpkg.CreateTestStudent(t, tc.db, "Multi", "Second", classSecond)

	groupA := testpkg.CreateTestEducationGroup(t, tc.db, fmt.Sprintf("Gruppe A %d", suffix))
	groupB := testpkg.CreateTestEducationGroup(t, tc.db, fmt.Sprintf("Gruppe B %d", suffix))
	assignStudentGroup(t, tc, third.ID, groupA.ID)
	assignStudentGroup(t, tc, fourth.ID, groupB.ID)

	defer testpkg.CleanupActivityFixtures(t, tc.db, third.ID, fourth.ID, second.ID, groupA.ID, groupB.ID)

	t.Run("two classes return both cohorts", func(t *testing.T) {
		ids, total := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(
			"school_class=%s,%s&page_size=50",
			url.QueryEscape(classThird), url.QueryEscape(classFourth),
		))

		assert.ElementsMatch(t, []int64{third.ID, fourth.ID}, ids)
		assert.Equal(t, 2, total, "the reported total must count the whole selection")
	})

	t.Run("a repeated parameter means the same as a comma list", func(t *testing.T) {
		ids, _ := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(
			"school_class=%s&school_class=%s&page_size=50",
			url.QueryEscape(classThird), url.QueryEscape(classFourth),
		))

		assert.ElementsMatch(t, []int64{third.ID, fourth.ID}, ids)
	})

	t.Run("a single class still filters exactly as before", func(t *testing.T) {
		ids, total := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(
			"school_class=%s&page_size=50", url.QueryEscape(classThird),
		))

		assert.Equal(t, []int64{third.ID}, ids)
		assert.Equal(t, 1, total)
	})

	t.Run("two groups return both rosters", func(t *testing.T) {
		ids, _ := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(
			"group_id=%d,%d&page_size=50", groupA.ID, groupB.ID,
		))

		assert.ElementsMatch(t, []int64{third.ID, fourth.ID}, ids)
	})

	t.Run("two grades cover both years", func(t *testing.T) {
		// The grade filter is not class-unique, so other children of the shared
		// test database legitimately share these years: assert membership, not
		// the exact set.
		ids, _ := listMultiFilterStudentIDs(t, tc, "grade_level=3,4&page_size=1000")

		assert.Contains(t, ids, third.ID)
		assert.Contains(t, ids, fourth.ID)
		assert.NotContains(t, ids, second.ID, "grade 2 is outside the selection")
	})
}
