package enrollment

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

func TestParseClassRosterExportRequestAllClasses(t *testing.T) {
	t.Run("accepts all_classes without school_class", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/enrollment/admin/reports/class-roster/export", strings.NewReader(`{
			"format":"pdf",
			"filters":{"phase_id":42,"all_classes":true}
		}`))

		format, filters, err := parseClassRosterExportRequest(req)

		require.NoError(t, err)
		assert.Equal(t, listexport.FormatPDF, format)
		assert.True(t, filters.AllClasses)
		assert.Empty(t, filters.SchoolClass)
	})

	t.Run("rejects all_classes combined with school_class", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/enrollment/admin/reports/class-roster/export", strings.NewReader(`{
			"format":"pdf",
			"filters":{"phase_id":42,"school_class":"1a","all_classes":true}
		}`))

		_, _, err := parseClassRosterExportRequest(req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "mutually exclusive")
	})

	t.Run("still rejects neither", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/enrollment/admin/reports/class-roster/export", strings.NewReader(`{
			"format":"pdf",
			"filters":{"phase_id":42}
		}`))

		_, _, err := parseClassRosterExportRequest(req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "school_class is required")
	})
}

func allClassesTestReport() *enrollmentService.ClassRosterReport {
	return &enrollmentService.ClassRosterReport{
		Phase: enrollmentService.CareUsagePhase{ID: 42, Name: "Schuljahr 2026/27"},
		Filters: enrollmentService.ClassRosterAppliedFilters{
			PhaseID:    42,
			AllClasses: true,
			Status:     "approved",
		},
		Totals: enrollmentService.ClassRosterTotals{Students: 3},
		Rows: []enrollmentService.ClassRosterRow{
			{StudentID: 1, FirstName: "Mila", LastName: "Anders", SchoolClass: "1a"},
			{StudentID: 2, FirstName: "Finn", LastName: "Becker", SchoolClass: "1a"},
			{StudentID: 3, FirstName: "Ida", LastName: "Conrad", SchoolClass: "2b"},
		},
	}
}

func TestBuildClassRosterTableDocumentAllClassesInsertsGroupHeadings(t *testing.T) {
	doc := buildClassRosterTableDocument(allClassesTestReport())

	var got []string
	for _, row := range doc.Rows {
		if row.GroupTitle != "" {
			got = append(got, row.GroupTitle)
		}
	}
	assert.Equal(t, []string{"Klasse 1a", "Klasse 2b"}, got)
	require.Len(t, doc.Rows, 5)
	assert.Equal(t, "Klassenlisten - Schuljahr 2026/27", doc.Title)
	assert.Contains(t, doc.Filters, "Klasse: Alle Klassen")
}

func TestBuildClassRosterTableDocumentMergesClassLabelVariants(t *testing.T) {
	report := allClassesTestReport()
	report.Rows = []enrollmentService.ClassRosterRow{
		{StudentID: 1, FirstName: "Mila", LastName: "Anders", SchoolClass: "1a"},
		{StudentID: 2, FirstName: "Finn", LastName: "Becker", SchoolClass: "1A"},
		{StudentID: 3, FirstName: "Ida", LastName: "Conrad", SchoolClass: "2b"},
	}

	doc := buildClassRosterTableDocument(report)

	var headings []string
	for _, row := range doc.Rows {
		if row.GroupTitle != "" {
			headings = append(headings, row.GroupTitle)
		}
	}
	// "1a" and "1A" are one logical class: one heading, first-seen label.
	assert.Equal(t, []string{"Klasse 1a", "Klasse 2b"}, headings)
	require.Len(t, doc.Rows, 5)
}

func TestBuildClassRosterTableDocumentSingleClassHasNoGroupHeadings(t *testing.T) {
	report := allClassesTestReport()
	report.Filters.AllClasses = false
	report.Filters.SchoolClass = "1a"
	report.Rows = report.Rows[:2]

	doc := buildClassRosterTableDocument(report)

	for _, row := range doc.Rows {
		assert.Empty(t, row.GroupTitle)
	}
	assert.Equal(t, "Klassenliste 1a - Schuljahr 2026/27", doc.Title)
}

func TestBuildClassRosterExportFileAllClassesFilename(t *testing.T) {
	file, err := buildClassRosterExportFile(listexport.NewService(), allClassesTestReport(), listexport.FormatPDF)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(file.Filename, "klassenlisten"), "filename = %q", file.Filename)
}
