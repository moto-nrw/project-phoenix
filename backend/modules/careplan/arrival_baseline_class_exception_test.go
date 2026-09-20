package careplan_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyClassExceptionReplacesTheCareDayRowOnly(t *testing.T) {
	t.Parallel()

	notes := "eigene Notiz"
	week := careplan.ArrivalWeek{
		int(time.Monday): {
			StudentID:       7,
			Weekday:         int(time.Monday),
			ExpectedArrival: mustClock(t, "13:30"),
			Notes:           &notes,
			Source:          careplan.ScheduleSourceStaff,
		},
	}

	source := week[int(time.Monday)]
	week[int(time.Monday)] = projectClassException(
		week[int(time.Monday)],
		&careplan.ArrivalBaselineException{SchoolClass: "4a", ArrivalTime: mustClock(t, "12:45"), Label: "Klasse 4a: Unterricht fällt aus"},
	)

	row := week[int(time.Monday)]
	require.NotNil(t, row)
	assert.Equal(t, "12:45", row.ExpectedArrival.Format("15:04"))
	assert.Equal(t, careplan.ScheduleSourceClassException, row.Source)
	assert.Equal(t, "4a", row.SourceClass)
	assert.Equal(t, "Klasse 4a: Unterricht fällt aus", row.SourceLabel)
	require.NotNil(t, row.Notes)
	assert.Equal(t, "Klasse 4a: Unterricht fällt aus, eigene Notiz", *row.Notes)
	assert.Equal(t, source.StudentID, row.StudentID)
	assert.NotSame(t, source, row, "a date-specific exception must not mutate the recurring plan")
	assert.Equal(t, "eigene Notiz", *source.Notes)
}

func TestClassExceptionRowSkipsDaysWithoutCare(t *testing.T) {
	t.Parallel()

	week := careplan.ArrivalWeek{}
	assert.Nil(t, projectClassException(nil, &careplan.ArrivalBaselineException{SchoolClass: "4a", ArrivalTime: mustClock(t, "12:45")}))
	assert.Nil(t, projectClassException(nil, nil))
	assert.Empty(t, week)
}

func mustClock(t *testing.T, hhmm string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", hhmm)
	require.NoError(t, err)
	return timezone.NormalizeWallClock(parsed)
}

func projectClassException(row *careplan.ArrivalSchedule, exception *careplan.ArrivalBaselineException) *careplan.ArrivalSchedule {
	date := timezone.NewDate(2031, time.February, 3)
	projection := &careplan.ArrivalBaselineProjection{
		WeeklyByStudentDate:          careplan.ArrivalPlansByStudent{7: {date: {1: row}}},
		ClassExceptionsByStudentDate: map[int64]careplan.ClassArrivalExceptionsByDate{7: {}},
	}
	if exception != nil {
		projection.ClassExceptionsByStudentDate[7][date] = exception
	}
	return projection.ForDate(7, date)
}
