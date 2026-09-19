package enrollment

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// fakeClassListEntries serves in-memory class-list entries (#2382) through
// the reader slice the report consumes.
type fakeClassListEntries struct {
	entries []ClassListEntry
}

func (r *fakeClassListEntries) ListClassListEntries(_ context.Context, schoolClass string) ([]ClassListEntry, error) {
	if schoolClass == "" {
		return r.entries, nil
	}
	key := strings.ToLower(strings.TrimSpace(schoolClass))
	var out []ClassListEntry
	for _, entry := range r.entries {
		if strings.ToLower(strings.TrimSpace(entry.SchoolClass)) == key {
			out = append(out, entry)
		}
	}
	return out, nil
}

func classListEntry(id int64, firstName, lastName, schoolClass string) ClassListEntry {
	return ClassListEntry{
		ID:          id,
		FirstName:   firstName,
		LastName:    lastName,
		SchoolClass: schoolClass,
	}
}

func TestClassRosterMergesClassListEntries(t *testing.T) {
	t.Parallel()

	svc := allClassesTestService()
	svc.ClassListEntries = &fakeClassListEntries{entries: []ClassListEntry{
		classListEntry(101, "Zoe", "Aalders", "1a"),
		classListEntry(102, "Ben", "Zorn", "3c"),
	}}

	report, err := svc.ClassRoster(context.Background(), ClassRosterFilters{PhaseID: 55, AllClasses: true})
	require.NoError(t, err)

	// 4 students + 2 entries; the 3c entry creates its own class section even
	// though no student carries that class.
	require.Len(t, report.Rows, 6)
	assert.Equal(t, 6, report.Totals.Students)
	assert.Equal(t, 2, report.Totals.ListEntries)

	got := make([][2]string, 0, len(report.Rows))
	for _, row := range report.Rows {
		got = append(got, [2]string{row.SchoolClass, row.LastName})
	}
	assert.Equal(t, [][2]string{
		{"1a", "Aalders"}, // list entry, alphabetically first in 1a
		{"1a", "Anders"},
		{"1a", "Becker"},
		{"2b", "Dreyer"},
		{"3c", "Zorn"}, // entry-only class gets its own section
		{"10a", "Conrad"},
	}, got)

	for _, row := range report.Rows {
		if row.ListEntry {
			assert.Zero(t, row.StudentID)
			assert.False(t, row.Registered)
			assert.Equal(t, ClassListEntryNoCareLabel, row.EnrollmentSummary)
			assert.Empty(t, row.CareDays)
		}
	}
}

func TestClassRosterSingleClassMergesOnlyThatClass(t *testing.T) {
	t.Parallel()

	svc := allClassesTestService()
	svc.ClassListEntries = &fakeClassListEntries{entries: []ClassListEntry{
		classListEntry(101, "Zoe", "Aalders", "1a"),
		classListEntry(102, "Ben", "Zorn", "3c"),
	}}

	report, err := svc.ClassRoster(context.Background(), ClassRosterFilters{PhaseID: 55, SchoolClass: "1a"})
	require.NoError(t, err)

	require.Len(t, report.Rows, 3)
	assert.Equal(t, 1, report.Totals.ListEntries)
	assert.Equal(t, "Aalders", report.Rows[0].LastName)
	assert.True(t, report.Rows[0].ListEntry)
}

func TestBuildClassDayReportListEntryProjection(t *testing.T) {
	t.Parallel()

	rows := []ClassRosterRow{
		{
			StudentID:  21,
			FirstName:  "Mila",
			LastName:   "Anders",
			Registered: true,
			OfferingsByDay: map[string][]string{
				"wed": {"Ganztag"},
			},
		},
		{
			ListEntry:         true,
			ListEntryID:       101,
			FirstName:         "Zoe",
			LastName:          "Aalders",
			EnrollmentSummary: ClassListEntryNoCareLabel,
		},
	}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr", rows, newClassDayFacts())

	require.Len(t, report.Rows, 2)
	entryRow := report.Rows[1]
	require.True(t, entryRow.ListEntry)
	assert.Equal(t, int64(101), entryRow.ListEntryID)
	assert.Zero(t, entryRow.StudentID)
	assert.False(t, entryRow.StaysToday)
	// "Keine Betreuung" is the whole statement: no departure text at all —
	// especially not "Keine Angabe", which would read as a plan gap.
	assert.Empty(t, entryRow.Departure)

	assert.Equal(t, 2, report.Totals.Students)
	assert.Equal(t, 1, report.Totals.Staying)
	// The entry is NOT "geht nach Hause" — it lands in its own neutral
	// "Keine Betreuung" bucket (#2399 review).
	assert.Equal(t, 0, report.Totals.Leaving)
	assert.Equal(t, 1, report.Totals.ListEntries)
}
