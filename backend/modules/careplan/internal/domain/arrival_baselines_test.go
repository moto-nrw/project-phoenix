package domain

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassArrivalBaselineReadsRetainedWeekdayKeys(t *testing.T) {
	t.Parallel()
	values := map[string]string{" MON ": "12:15", "tue": "invalid", "fri": "11:45", "sat": "10:00"}
	plan := ClassArrivalBaselineFromTimes(" 3A ", values)
	require.Equal(t, " 3A ", plan.SchoolClass)
	require.Len(t, plan.ArrivalTimes, 2)
	monday, ok := plan.TimeForWeekday(1)
	require.True(t, ok)
	require.Equal(t, "12:15", monday.Format("15:04"))
	_, ok = plan.TimeForWeekday(2)
	require.False(t, ok, "an invalid clock must not create an expected arrival")
	_, ok = plan.TimeForWeekday(6)
	require.False(t, ok, "class arrival baselines apply Monday through Friday")
	require.Contains(t, values, " MON ", "reading must not normalize persisted data in place")
}

func TestEffectiveArrivalRowKeepsBookingCareDayWithoutTime(t *testing.T) {
	t.Parallel()

	row := EffectiveArrivalRow(42, 1, nil, nil)

	require.NotNil(t, row)
	assert.Equal(t, int64(42), row.StudentID)
	assert.Equal(t, 1, row.Weekday)
	assert.True(t, row.ExpectedArrival.IsZero())
	assert.Empty(t, row.Source)
}

func TestEffectiveArrivalRowDoesNotInventClassSourceWithoutClassTime(t *testing.T) {
	t.Parallel()

	stored := &careplan.ArrivalSchedule{
		StudentID: 42,
		Weekday:   1,
	}
	row := EffectiveArrivalRow(42, 1, stored, nil)

	require.NotNil(t, row)
	assert.Empty(t, row.Source)
	assert.Empty(t, row.SourceClass)
}
