package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func keyDateRow(school int64, month time.Month, students, terminals int) traegerview.SchoolKeyDateCount {
	return traegerview.SchoolKeyDateCount{SchoolID: school, KeyDateCount: traegerview.KeyDateCount{
		Period: calendar.NewDate(2026, month, 1), KeyDate: calendar.NewDate(2026, month, 15),
		ActiveStudents: students, ActiveTerminals: terminals,
	}}
}

func TestKeyDateCountsSumEachMonthOverTheTraegerSchools(t *testing.T) {
	t.Parallel()
	const bach, linden, foreign int64 = 1, 2, 3
	schools := []traegerview.School{{ID: bach, Name: "OGS Am Bach"}, {ID: linden, Name: "OGS Lindenweg"}}
	got := keyDateCounts(schools, []traegerview.SchoolKeyDateCount{
		keyDateRow(bach, time.October, 30, 2), keyDateRow(linden, time.October, 12, 1), keyDateRow(foreign, time.October, 99, 9),
		keyDateRow(bach, time.September, 28, 2),
	})

	require.Len(t, got.Schools, 2)
	assert.Len(t, got.Schools[0].Counts, 2)
	assert.Len(t, got.Schools[1].Counts, 1)
	require.Len(t, got.Months, 2)
	assert.Equal(t, traegerview.KeyDateMonth{
		Period: calendar.NewDate(2026, time.October, 1), KeyDate: calendar.NewDate(2026, time.October, 15),
		ActiveStudents: 42, ActiveTerminals: 3, Schools: 2,
	}, got.Months[0], "a row of a school outside the Träger enters no sum")
	assert.Equal(t, 1, got.Months[1].Schools, "a month counts only the schools captured in it")
}

func TestKeyDateCountsWithoutRowsAnswerEmptyLists(t *testing.T) {
	t.Parallel()
	got := keyDateCounts([]traegerview.School{{ID: 1}}, nil)

	require.Len(t, got.Schools, 1)
	assert.NotNil(t, got.Schools[0].Counts)
	assert.NotNil(t, got.Months)
}
