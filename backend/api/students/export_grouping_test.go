package students

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupExportResponsesByClassKeepsWithinClassOrder(t *testing.T) {
	students := []StudentResponse{
		{ID: 1, FirstName: "Emma", LastName: "Anders", SchoolClass: "10a"},
		{ID: 2, FirstName: "Finn", LastName: "Becker", SchoolClass: "2a"},
		{ID: 3, FirstName: "Ida", LastName: "Conrad", SchoolClass: ""},
		{ID: 4, FirstName: "Mila", LastName: "Anders", SchoolClass: "2a"},
	}

	groupExportResponsesByClass(students)

	gotIDs := []int64{students[0].ID, students[1].ID, students[2].ID, students[3].ID}
	// "2a" before "10a" (numeric grade order); empty class last; the two
	// 2a students keep their prior relative order (stable sort).
	assert.Equal(t, []int64{2, 4, 1, 3}, gotIDs)
}

func TestBuildGroupedExportRowsInsertsHeadingsAtBoundaries(t *testing.T) {
	students := []StudentResponse{
		{ID: 1, FirstName: "Finn", LastName: "Becker", SchoolClass: "1a"},
		{ID: 2, FirstName: "Mila", LastName: "Anders", SchoolClass: "1a"},
		{ID: 3, FirstName: "Emma", LastName: "Anders", SchoolClass: "1b"},
		{ID: 4, FirstName: "Ida", LastName: "Conrad", SchoolClass: ""},
	}

	rows := buildGroupedExportRows(students, map[int64]weeklySchedule{}, map[int64]string{}, testExportDate, true)

	require.Len(t, rows, 7)
	assert.Equal(t, "Klasse 1a", rows[0].GroupTitle)
	assert.Empty(t, rows[1].GroupTitle)
	assert.Empty(t, rows[2].GroupTitle)
	assert.Equal(t, "Klasse 1b", rows[3].GroupTitle)
	assert.Empty(t, rows[4].GroupTitle)
	assert.Equal(t, "Ohne Klasse", rows[5].GroupTitle)
	assert.Empty(t, rows[6].GroupTitle)
	assert.Equal(t, "Finn Becker", rows[1].Values[listexport.ColumnName])
	assert.Equal(t, "Ida Conrad", rows[6].Values[listexport.ColumnName])
}

func TestBuildGroupedExportRowsMergesClassLabelVariants(t *testing.T) {
	// Case/spacing variants of one logical class arrive sort-adjacent
	// (the comparator treats them as equal) and must share one heading.
	students := []StudentResponse{
		{ID: 1, FirstName: "Mila", LastName: "Anders", SchoolClass: "1a"},
		{ID: 2, FirstName: "Finn", LastName: "Becker", SchoolClass: "1A"},
		{ID: 3, FirstName: "Emma", LastName: "Conrad", SchoolClass: "1 a"},
		{ID: 4, FirstName: "Ida", LastName: "Dreyer", SchoolClass: "1b"},
	}

	rows := buildGroupedExportRows(students, map[int64]weeklySchedule{}, map[int64]string{}, testExportDate, true)

	var headings []string
	for _, row := range rows {
		if row.GroupTitle != "" {
			headings = append(headings, row.GroupTitle)
		}
	}
	assert.Equal(t, []string{"Klasse 1a", "Klasse 1b"}, headings)
	require.Len(t, rows, 6)
}

func TestBuildGroupedExportRowsSingleClassHasOneHeading(t *testing.T) {
	students := []StudentResponse{
		{ID: 1, FirstName: "Finn", LastName: "Becker", SchoolClass: "1a"},
		{ID: 2, FirstName: "Mila", LastName: "Anders", SchoolClass: "1a"},
	}

	rows := buildGroupedExportRows(students, map[int64]weeklySchedule{}, map[int64]string{}, testExportDate, true)

	require.Len(t, rows, 3)
	headings := 0
	for _, row := range rows {
		if row.GroupTitle != "" {
			headings++
		}
	}
	assert.Equal(t, 1, headings)
}

func TestExportFilterLabelsIncludesGroupByClass(t *testing.T) {
	labels := exportFilterLabels(studentExportFilters{GroupByClass: true})
	assert.Contains(t, labels, "Nach Klassen getrennt")
}
