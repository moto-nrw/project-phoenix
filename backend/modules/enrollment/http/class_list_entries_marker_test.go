package enrollmenthttp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The class-roster table has no status column, so a class-list-only entry
// (#2382) carries its "Keine Betreuung" marker in the name cell and renders
// "—" in every weekday cell like a non-care day.
func TestClassRosterTableDocumentMarksListEntries(t *testing.T) {
	t.Parallel()

	report := &capability.ClassRosterReport{
		Filters: capability.ClassRosterAppliedFilters{SchoolClass: "1a"},
		Totals:  capability.ClassRosterTotals{Students: 2, Registered: 1, ListEntries: 1},
		Rows: []capability.ClassRosterRow{
			{
				ListEntry:         true,
				ListEntryID:       101,
				FirstName:         "Zoe",
				LastName:          "Aalders",
				SchoolClass:       "1a",
				EnrollmentSummary: capability.ClassListEntryNoCareLabel,
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
	assert.Equal(t, "Zoe Aalders (Keine Betreuung)", entryRow.Values[lists.ColumnName])
	assert.Equal(t, "—", entryRow.Values[lists.ColumnWeeklyMonday])
	assert.Equal(t, "—", entryRow.Values[lists.ColumnWeeklyFriday])

	studentRow := doc.Rows[1]
	assert.Equal(t, "Mila Anders", studentRow.Values[lists.ColumnName])
	assert.Equal(t, "15:00 Uhr", studentRow.Values[lists.ColumnWeeklyMonday])

	assert.Equal(t, "2 Kinder, 1 angemeldet, 1 ohne Betreuung", classRosterSubtitle(report))
}

// newTestDocumentRenderer is the Document Rendering renderer the root hands
// the export routes; the contract's list renderer renders every layout.
func newTestDocumentRenderer() lists.DocumentRenderer {
	renderer, ok := lists.NewRenderer().(lists.DocumentRenderer)
	if !ok {
		panic("the list renderer no longer renders every layout")
	}
	return renderer
}
