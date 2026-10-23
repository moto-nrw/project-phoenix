package records_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
)

// Both class lists print the same columns in the same order (#4749); a
// source's own columns come after them.
func TestClassRosterColumnIDs(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []records.ColumnID{
		records.ColumnName,
		records.ColumnSchoolClass,
		records.ColumnWeeklyMonday,
		records.ColumnWeeklyTuesday,
		records.ColumnWeeklyWednesday,
		records.ColumnWeeklyThursday,
		records.ColumnWeeklyFriday,
	}, records.ClassRosterColumnIDs())
	assert.Equal(t, records.ClassRosterColumnIDs(), records.DefaultColumnsForPreset(records.PresetClassRoster),
		"the Kinderlisten-Vorlage prints the shared columns by default")
}

func TestClassRosterDayCell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		day  records.ClassRosterDay
		want string
	}{
		{"no care day ignores every other field", records.ClassRosterDay{Pickup: "14:30", Departure: "wird abgeholt"}, "—"},
		{"pickup and rule", records.ClassRosterDay{Care: true, Pickup: "14:30", Departure: "wird abgeholt"}, "14:30 Uhr, wird abgeholt"},
		{"pickup outranks offerings", records.ClassRosterDay{Care: true, Pickup: " 15:00 ", Offerings: []string{"Betreuung bis 16 Uhr"}}, "15:00 Uhr"},
		{"offerings stand in for a missing pickup", records.ClassRosterDay{Care: true, Offerings: []string{"Früh", "Spät"}, Departure: "fährt Bus"}, "Früh; Spät, fährt Bus"},
		{"care day without any time", records.ClassRosterDay{Care: true, Departure: "geht alleine"}, "Keine Abholzeit, geht alleine"},
		{"blank rule adds nothing", records.ClassRosterDay{Care: true, Pickup: "14:30", Departure: "  "}, "14:30 Uhr"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, records.ClassRosterDayCell(tt.day))
		})
	}
}

// The shared columns win over a source's extra values, and a class-list-only
// entry carries its marker in the name and "—" on every weekday even when a
// caller handed it a day.
func TestClassRosterRow(t *testing.T) {
	t.Parallel()

	row := records.ClassRosterRow(records.ClassRosterChild{
		Name:        " Mila Anders ",
		SchoolClass: "1a",
		Days:        [5]records.ClassRosterDay{{Care: true, Pickup: "14:30", Departure: "wird abgeholt"}},
		Extra: map[records.ColumnID]string{
			records.ColumnGroup: "Igel",
			records.ColumnName:  "overwritten",
		},
	})
	assert.Equal(t, "Mila Anders", row.Values[records.ColumnName])
	assert.Equal(t, "1a", row.Values[records.ColumnSchoolClass])
	assert.Equal(t, "Igel", row.Values[records.ColumnGroup])
	assert.Equal(t, "14:30 Uhr, wird abgeholt", row.Values[records.ColumnWeeklyMonday])
	assert.Equal(t, "—", row.Values[records.ColumnWeeklyFriday])

	entry := records.ClassRosterRow(records.ClassRosterChild{
		Name:        "Zoe Aalders",
		SchoolClass: "1a",
		NoCare:      true,
		Days:        [5]records.ClassRosterDay{{Care: true, Pickup: "14:30"}},
	})
	assert.Equal(t, "Zoe Aalders (Keine Betreuung)", entry.Values[records.ColumnName])
	for _, id := range records.ClassRosterWeekdayColumns {
		assert.Equal(t, "—", entry.Values[id], id)
	}
}

// A grouped list opens each class with one heading; "1a" and "1A" are one
// class under the first-seen label, and a child without a class collects
// under "Ohne Klasse". An ungrouped list has no headings.
func TestClassRosterRowsGroupsByClass(t *testing.T) {
	t.Parallel()

	children := []records.ClassRosterChild{
		{Name: "Ada", SchoolClass: ""},
		{Name: "Ben", SchoolClass: "1a"},
		{Name: "Cem", SchoolClass: "1A"},
		{Name: "Dua", SchoolClass: "Klasse 2b"},
	}

	grouped := records.ClassRosterRows(children, true)
	titles := []string{}
	names := []string{}
	for _, row := range grouped {
		if row.GroupTitle != "" {
			titles = append(titles, row.GroupTitle)
			continue
		}
		names = append(names, row.Values[records.ColumnName])
	}
	assert.Equal(t, []string{"Ohne Klasse", "Klasse 1a", "Klasse 2b"}, titles)
	assert.Equal(t, []string{"Ada", "Ben", "Cem", "Dua"}, names)

	flat := records.ClassRosterRows(children, false)
	require.Len(t, flat, len(children))
	for _, row := range flat {
		assert.Empty(t, row.GroupTitle)
	}
}
