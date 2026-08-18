package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/services/listexport"

	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// The class-roster table has no status column, so a class-list-only entry
// (#2382) carries its "Keine Betreuung" marker in the name cell and renders
// "—" in every weekday cell like a non-care day.
func TestClassRosterTableDocumentMarksListEntries(t *testing.T) {
	report := &enrollmentService.ClassRosterReport{
		Filters: enrollmentService.ClassRosterAppliedFilters{SchoolClass: "1a"},
		Totals:  enrollmentService.ClassRosterTotals{Students: 2, Registered: 1, ListEntries: 1},
		Rows: []enrollmentService.ClassRosterRow{
			{
				ListEntry:         true,
				ListEntryID:       101,
				FirstName:         "Zoe",
				LastName:          "Aalders",
				SchoolClass:       "1a",
				EnrollmentSummary: enrollmentService.ClassListEntryNoCareLabel,
			},
			{
				StudentID:   21,
				FirstName:   "Mila",
				LastName:    "Anders",
				SchoolClass: "1a",
				Registered:  true,
				CareDays:    []string{"mon"},
				PickupByDay: map[string]string{"mon": "15:00"},
			},
		},
	}

	doc := buildClassRosterTableDocument(report)
	require.Len(t, doc.Rows, 2)

	entryRow := doc.Rows[0]
	assert.Equal(t, "Zoe Aalders (Keine Betreuung)", entryRow.Values[listexport.ColumnName])
	assert.Equal(t, "—", entryRow.Values[listexport.ColumnWeeklyMonday])
	assert.Equal(t, "—", entryRow.Values[listexport.ColumnWeeklyFriday])

	studentRow := doc.Rows[1]
	assert.Equal(t, "Mila Anders", studentRow.Values[listexport.ColumnName])
	assert.Equal(t, "15:00 Uhr", studentRow.Values[listexport.ColumnWeeklyMonday])

	assert.Equal(t, "2 Kinder, 1 angemeldet, 1 ohne Betreuung", classRosterSubtitle(report))
}
