package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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
	t.Parallel()

	tc := setupStudentsRoute(t)

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

	t.Run("a group selection is paginated instead of returned whole", func(t *testing.T) {
		// The group-only fast path materializes both rosters in memory, so it has
		// to cut the requested page itself — otherwise a caller asking for one row
		// receives every child while the metadata promises a one-row page, and a
		// selection larger than the page size can never be walked (#2218 review).
		firstPage, total := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(
			"group_id=%d,%d&page=1&page_size=1", groupA.ID, groupB.ID,
		))
		require.Len(t, firstPage, 1, "page_size=1 must yield exactly one child")
		assert.Equal(t, 2, total, "the total still counts the whole selection")

		secondPage, _ := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(
			"group_id=%d,%d&page=2&page_size=1", groupA.ID, groupB.ID,
		))
		require.Len(t, secondPage, 1)

		assert.ElementsMatch(t, []int64{third.ID, fourth.ID}, append(firstPage, secondPage...),
			"the pages together must cover the selection without repeating a child")

		beyond, _ := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(
			"group_id=%d,%d&page=3&page_size=1", groupA.ID, groupB.ID,
		))
		assert.Empty(t, beyond, "a page past the end is empty, not a repeat of the last one")
	})

	t.Run("an extreme page number answers empty instead of failing", func(t *testing.T) {
		// (page-1)*page_size used to overflow before it was turned into an
		// offset, which is a negative slice index on the group fast path and a
		// negative SQL OFFSET on the standard path — both a 500 (#2218 review).
		beyond, total := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(
			"group_id=%d,%d&page=9223372036854775807&page_size=2", groupA.ID, groupB.ID,
		))
		assert.Empty(t, beyond)
		assert.Equal(t, 2, total, "the total still counts the whole selection")

		// Same request without the group fast path, so the clamp is exercised
		// against the SQL OFFSET as well.
		beyondSQL, _ := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(
			"school_class=%s&page=9223372036854775807&page_size=2", url.QueryEscape(classThird),
		))
		assert.Empty(t, beyondSQL)
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

	// The grade filter is answered in SQL (#2218 review), which means the
	// group-only fast path — which never runs that query — must stop taking the
	// shortcut as soon as a grade is named. Otherwise the group's whole roster
	// comes back with the year silently ignored.
	t.Run("a grade narrows a group selection", func(t *testing.T) {
		ids, total := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(
			"group_id=%d,%d&grade_level=3&page_size=50", groupA.ID, groupB.ID,
		))

		assert.Equal(t, []int64{third.ID}, ids, "the fourth-grader is outside the selection")
		assert.Equal(t, 1, total, "the count narrows with the grade, too")
	})

	// Paging only adds up to the whole selection if consecutive windows come out
	// of one stable row order. Without an ORDER BY, PostgreSQL may answer two
	// requests from two different orders, and a child is then listed twice while
	// another is never listed at all (#2218 review).
	t.Run("consecutive pages cover a class selection exactly once", func(t *testing.T) {
		seen := []int64{}
		for page := 1; page <= 2; page++ {
			ids, total := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(
				"school_class=%s,%s&page=%d&page_size=1",
				url.QueryEscape(classThird), url.QueryEscape(classFourth), page,
			))
			require.Len(t, ids, 1, "page %d must hold exactly one child", page)
			assert.Equal(t, 2, total)
			seen = append(seen, ids...)
		}

		assert.Equal(t, []int64{third.ID, fourth.ID}, seen,
			"the pages must walk the selection in id order, each child exactly once")
	})
}

// users.students.school_class is free text, so a class may carry the very
// character the multi-value filters separate on. Escaped, it stays one value
// end to end; unescaped it would silently become two filters matching nothing
// (#2218 review).
func TestListStudents_ClassNameContainingTheSeparator(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	suffix := time.Now().UnixNano()
	commaClass := fmt.Sprintf("A,B-%d", suffix)
	plainClass := fmt.Sprintf("A-%d", suffix)

	comma := testpkg.CreateTestStudent(t, tc.db, "Sep", "Comma", commaClass)
	plain := testpkg.CreateTestStudent(t, tc.db, "Sep", "Plain", plainClass)

	t.Run("an escaped comma selects the one class", func(t *testing.T) {
		ids, total := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(
			"school_class=%s&page_size=50",
			url.QueryEscape(strings.ReplaceAll(commaClass, ",", `\,`)),
		))

		assert.Equal(t, []int64{comma.ID}, ids)
		assert.Equal(t, 1, total)
	})

	t.Run("an unescaped comma still separates two classes", func(t *testing.T) {
		ids, _ := listMultiFilterStudentIDs(t, tc, fmt.Sprintf(
			"school_class=%s,%s&page_size=50",
			url.QueryEscape(plainClass), url.QueryEscape(commaClass),
		))

		// Membership, not the exact set: the fragments this splits into are
		// short enough that the shared test database may hold a class of that
		// name from another fixture.
		assert.Contains(t, ids, plain.ID)
		assert.NotContains(t, ids, comma.ID,
			`"A,B" read as two classes must not match the class actually called "A,B"`)
	})
}
