package enrollment

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestParseCareUsageExportRequestLayout(t *testing.T) {
	t.Run("defaults to detailed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/care-usage/export", strings.NewReader(`{
			"format": "pdf",
			"filters": {"phase_id": "42"}
		}`))

		_, params, err := parseCareUsageExportRequest(req)

		require.NoError(t, err)
		assert.Equal(t, careUsageLayoutDetailed, params.Layout)
	})

	t.Run("accepts compact", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/care-usage/export", strings.NewReader(`{
			"format": "docx",
			"layout": "compact",
			"filters": {"phase_id": "42"}
		}`))

		format, params, err := parseCareUsageExportRequest(req)

		require.NoError(t, err)
		assert.Equal(t, listexport.FormatDOCX, format)
		assert.Equal(t, careUsageLayoutCompact, params.Layout)
		assert.Equal(t, int64(42), params.PhaseID)
	})

	t.Run("rejects unknown layout", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/care-usage/export", strings.NewReader(`{
			"format": "pdf",
			"layout": "fancy",
			"filters": {"phase_id": "42"}
		}`))

		_, _, err := parseCareUsageExportRequest(req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported export layout")
	})
}

func compactLayoutTestReport() *enrollmentService.CareUsageReport {
	grade := int16(1)
	phone := "0151 2345678"
	return &enrollmentService.CareUsageReport{
		Phase: enrollmentService.CareUsagePhase{ID: 42, Name: "Schuljahr 2026/27"},
		Filters: enrollmentService.CareUsageAppliedFilters{
			PhaseID:    42,
			Status:     "approved",
			PickupTime: "14:30",
		},
		Totals: enrollmentService.CareUsageTotals{Children: 3},
		Rows: []enrollmentService.CareUsageRow{
			{
				RequestID:         1,
				ChildFirstName:    "Ida",
				ChildLastName:     "Conrad",
				TargetSchoolClass: testpkg.StrPtr("2b"),
				EffectiveDays:     []string{"mon", "wed"},
				PickupByDay:       map[string]string{"mon": "14:30", "wed": "14:30"},
				GuardianFirstName: "Carla",
				GuardianLastName:  "Conrad",
				GuardianEmail:     "carla@example.org",
				GuardianPhone:     &phone,
			},
			{
				RequestID:         2,
				ChildFirstName:    "Mila",
				ChildLastName:     "Anders",
				TargetSchoolClass: testpkg.StrPtr("1a"),
				EffectiveDays:     []string{"mon", "tue"},
				PickupByDay:       map[string]string{"mon": "14:30"},
				Offerings: []enrollmentService.CareUsageRowOffering{
					{ID: 7, Name: "Betreuung bis 16:00 Uhr", Days: []string{"tue"}},
				},
				GuardianFirstName: "Anna",
				GuardianLastName:  "Anders",
				GuardianEmail:     "anna@example.org",
			},
			{
				RequestID:         3,
				ChildFirstName:    "Finn",
				ChildLastName:     "Becker",
				TargetGradeLevel:  &grade,
				EffectiveDays:     []string{"fri"},
				PickupByDay:       map[string]string{},
				GuardianFirstName: "Bernd",
				GuardianLastName:  "Becker",
				GuardianEmail:     "bernd@example.org",
			},
		},
	}
}

func TestBuildCareUsageCompactTableDocumentOneRowPerChildSortedByClass(t *testing.T) {
	doc := buildCareUsageCompactTableDocument(compactLayoutTestReport())

	var headings []string
	var names []string
	for _, row := range doc.Rows {
		if row.GroupTitle != "" {
			headings = append(headings, row.GroupTitle)
			continue
		}
		names = append(names, row.Values[listexport.ColumnName])
	}
	// Three classes ("1. Klasse", "1a", "2b") -> grouped output with one
	// heading per class and one data row per child, class-sorted.
	assert.Len(t, names, 3)
	assert.Equal(t, []string{"1. Klasse", "Klasse 1a", "Klasse 2b"}, headings)
	assert.Equal(t, []string{"Finn Becker", "Mila Anders", "Ida Conrad"}, names)
	assert.Equal(t, "Auswertung Schuljahr 2026/27", doc.Title)
	assert.Equal(t, "3 Kinder", doc.Subtitle)
	assert.Contains(t, doc.Filters, "Status: Angenommen")
	assert.Contains(t, doc.Filters, "Gehzeit: 14:30")
}

func TestBuildCareUsageCompactTableDocumentWeeklyCells(t *testing.T) {
	doc := buildCareUsageCompactTableDocument(compactLayoutTestReport())

	byName := map[string]map[listexport.ColumnID]string{}
	for _, row := range doc.Rows {
		if row.GroupTitle == "" {
			byName[row.Values[listexport.ColumnName]] = row.Values
		}
	}

	mila := byName["Mila Anders"]
	require.NotNil(t, mila)
	assert.Equal(t, "14:30 Uhr", mila[listexport.ColumnWeeklyMonday])
	// No pickup time on Tuesday -> offering-name fallback.
	assert.Equal(t, "Betreuung bis 16:00 Uhr", mila[listexport.ColumnWeeklyTuesday])
	assert.Equal(t, "—", mila[listexport.ColumnWeeklyWednesday])
	assert.Equal(t, "1a", mila[listexport.ColumnSchoolClass])
	assert.Equal(t, "Anna Anders (anna@example.org)", mila[listexport.ColumnGuardianContacts])

	finn := byName["Finn Becker"]
	require.NotNil(t, finn)
	// Care day without pickup time and without offering names.
	assert.Equal(t, "Keine Abholzeit", finn[listexport.ColumnWeeklyFriday])
	assert.Equal(t, "1. Klasse", finn[listexport.ColumnSchoolClass])

	ida := byName["Ida Conrad"]
	require.NotNil(t, ida)
	assert.Equal(t, "Carla Conrad (carla@example.org, 0151 2345678)", ida[listexport.ColumnGuardianContacts])
}

func TestBuildCareUsageCompactTableDocumentSingleClassHasNoHeadings(t *testing.T) {
	report := compactLayoutTestReport()
	report.Rows = report.Rows[:1]

	doc := buildCareUsageCompactTableDocument(report)

	require.Len(t, doc.Rows, 1)
	assert.Empty(t, doc.Rows[0].GroupTitle)
}

func TestBuildCareUsageCompactTableDocumentMergesClassLabelVariants(t *testing.T) {
	report := compactLayoutTestReport()
	report.Rows[0].TargetSchoolClass = testpkg.StrPtr("1A")
	report.Rows[2].TargetSchoolClass = testpkg.StrPtr("1 a")
	report.Rows[2].TargetGradeLevel = nil

	doc := buildCareUsageCompactTableDocument(report)

	var headings []string
	for _, row := range doc.Rows {
		if row.GroupTitle != "" {
			headings = append(headings, row.GroupTitle)
		}
	}
	// "1A", "1a", "1 a" are one logical class -> no grouping at all.
	assert.Empty(t, headings)
	assert.Len(t, doc.Rows, 3)
}

func TestBuildCareUsageExportLayoutDispatch(t *testing.T) {
	svc := listexport.NewService()
	report := compactLayoutTestReport()

	t.Run("compact pdf and docx render the table document", func(t *testing.T) {
		for _, format := range []listexport.Format{listexport.FormatPDF, listexport.FormatDOCX} {
			file, err := buildCareUsageExport(svc, careUsageExportPayload{report: report, layout: careUsageLayoutCompact}, format)
			require.NoError(t, err, format)
			assert.Contains(t, file.Filename, "kompakt", format)
			assert.NotEmpty(t, file.Data, format)
		}
	})

	t.Run("compact xlsx stays tabular", func(t *testing.T) {
		file, err := buildCareUsageExport(svc, careUsageExportPayload{report: report, layout: careUsageLayoutCompact}, listexport.FormatXLSX)
		require.NoError(t, err)
		assert.NotContains(t, file.Filename, "kompakt")
		assert.True(t, strings.HasSuffix(file.Filename, ".xlsx"), "filename = %q", file.Filename)
	})

	t.Run("detailed keeps the record layout", func(t *testing.T) {
		file, err := buildCareUsageExport(svc, careUsageExportPayload{report: report, layout: careUsageLayoutDetailed}, listexport.FormatPDF)
		require.NoError(t, err)
		assert.NotContains(t, file.Filename, "kompakt")
		assert.NotEmpty(t, file.Data)
	})
}
