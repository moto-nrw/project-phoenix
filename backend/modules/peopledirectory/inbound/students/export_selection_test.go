package students

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
)

// The selection export (#3834) prints only the children marked in a list.
func TestApplyExportFiltersKeepsOnlySelectedStudents(t *testing.T) {
	t.Parallel()

	got := applyExportFilters(birthdayFixtures(), studentExportFilters{StudentIDs: []string{"102", "105"}}, lists.PresetOGSWeekly, testExportDate)

	assert.Equal(t, []int64{102, 105}, exportedIDs(got))
}

// A selection narrows the other filters and never widens them: a marked child
// another filter drops stays out of the document.
func TestApplyExportFiltersSelectionIntersectsOtherFilters(t *testing.T) {
	t.Parallel()

	got := applyExportFilters(birthdayFixtures(), studentExportFilters{
		StudentIDs: []string{"101", "103"},
		Months:     []string{"09"},
	}, lists.PresetOGSWeekly, testExportDate)

	assert.Equal(t, []int64{101}, exportedIDs(got))
}

func TestApplyExportFiltersWithoutSelectionKeepsEveryStudent(t *testing.T) {
	t.Parallel()

	got := applyExportFilters(birthdayFixtures(), studentExportFilters{}, lists.PresetOGSWeekly, testExportDate)

	assert.Len(t, got, len(birthdayFixtures()))
}

func TestParseExportStudentIDs(t *testing.T) {
	t.Parallel()

	t.Run("empty list means no selection", func(t *testing.T) {
		ids, err := parseExportStudentIDs(nil)
		require.NoError(t, err)
		assert.Nil(t, ids)
	})

	t.Run("accepts trimmed numeric ids", func(t *testing.T) {
		ids, err := parseExportStudentIDs([]string{"7", " 12 "})
		require.NoError(t, err)
		assert.Equal(t, map[int64]bool{7: true, 12: true}, ids)
	})

	// A silently skipped id would print a list that misses a marked child.
	for _, value := range []string{"", "abc", "0", "-3", "1.5"} {
		t.Run("rejects "+value, func(t *testing.T) {
			_, err := parseExportStudentIDs([]string{value})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid student id")
		})
	}

	t.Run("rejects a selection over the export cap", func(t *testing.T) {
		values := make([]string, studentExportPageSize+1)
		for i := range values {
			values[i] = "1"
		}
		_, err := parseExportStudentIDs(values)
		require.Error(t, err)
	})
}

func TestDecodeStudentExportRequestRejectsInvalidStudentID(t *testing.T) {
	t.Parallel()

	body := `{"filters":{"student_ids":["12","x"]}}`
	req := httptest.NewRequest("POST", "/students/export", strings.NewReader(body))

	_, err := decodeStudentExportRequest(req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid student id")
}

func TestExportFilterLabelsNameTheSelection(t *testing.T) {
	t.Parallel()

	labels := exportFilterLabelsForDate(studentExportFilters{StudentIDs: []string{"1", "2"}}, testExportDate, true)

	assert.Contains(t, labels, "Nur ausgewählte Kinder")
}
