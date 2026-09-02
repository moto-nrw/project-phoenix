package schedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

func classExceptionAt(class, hhmm, reason string) *scheduleModel.ClassArrivalException {
	parsed, _ := time.Parse("15:04", hhmm)
	row := &scheduleModel.ClassArrivalException{SchoolClass: class, ArrivalTime: parsed}
	if reason != "" {
		row.Reason = &reason
	}
	return row
}

func TestApplyClassExceptionReplacesTheCareDayRowOnly(t *testing.T) {
	t.Parallel()

	monday := timezone.NewDate(2027, 3, 1)
	notes := "eigene Notiz"
	week := ArrivalWeek{
		scheduleModel.WeekdayMonday: {
			StudentID:       7,
			Weekday:         scheduleModel.WeekdayMonday,
			ExpectedArrival: mustClock(t, "13:30"),
			Notes:           &notes,
			Source:          scheduleModel.ArrivalScheduleSourceStaff,
		},
	}

	applyClassException(week, monday, classExceptionAt("4a", "12:45", "Unterricht fällt aus"))

	row := week[scheduleModel.WeekdayMonday]
	require.NotNil(t, row)
	assert.Equal(t, "12:45", row.ExpectedArrival.Format("15:04"))
	assert.Equal(t, scheduleModel.ArrivalScheduleSourceClassException, row.Source)
	assert.Equal(t, "4a", row.SourceClass)
	assert.Equal(t, "Klasse 4a: Unterricht fällt aus", row.SourceLabel)
	require.NotNil(t, row.Notes)
	assert.Equal(t, "Klasse 4a: Unterricht fällt aus, eigene Notiz", *row.Notes)
	assert.Equal(t, int64(7), row.StudentID)
}

func TestApplyClassExceptionSkipsDaysWithoutCare(t *testing.T) {
	t.Parallel()

	week := ArrivalWeek{}
	applyClassException(week, timezone.NewDate(2027, 3, 1), classExceptionAt("4a", "12:45", ""))
	assert.Empty(t, week)

	applyClassException(week, timezone.NewDate(2027, 3, 1), nil)
	assert.Empty(t, week)
}

func TestAttachClassExceptionOnlyForProjectedClassRows(t *testing.T) {
	t.Parallel()

	clock := mustClock(t, "12:45")

	effective := &EffectiveArrivalTime{ArrivalTime: &clock}
	attachClassException(effective, &scheduleModel.StudentArrivalSchedule{
		Source:      scheduleModel.ArrivalScheduleSourceClassException,
		SourceClass: "4a",
		SourceLabel: "Klasse 4a: Unterricht fällt aus",
	})
	require.NotNil(t, effective.ClassException)
	assert.Equal(t, "4a", effective.ClassException.SchoolClass)
	assert.Equal(t, "12:45", effective.ClassException.ArrivalTime)
	assert.Equal(t, "Klasse 4a: Unterricht fällt aus", effective.ClassException.Label)

	overridden := &EffectiveArrivalTime{ArrivalTime: &clock, IsException: true}
	attachClassException(overridden, &scheduleModel.StudentArrivalSchedule{Source: scheduleModel.ArrivalScheduleSourceClassException})
	assert.Nil(t, overridden.ClassException, "a per-child day exception hides the class one")

	regular := &EffectiveArrivalTime{ArrivalTime: &clock}
	attachClassException(regular, &scheduleModel.StudentArrivalSchedule{Source: scheduleModel.ArrivalScheduleSourceClassSchedule})
	assert.Nil(t, regular.ClassException)

	attachClassException(regular, nil)
	assert.Nil(t, regular.ClassException)
}

func TestAppendArrivalWarningsAddsTheClassExceptionLine(t *testing.T) {
	t.Parallel()

	inst := &scheduleModel.ActivityInstance{StartTime: mustClock(t, "12:45")}
	clock := mustClock(t, "12:45")
	warnings := map[int64][]OperationRosterWarning{}

	appendArrivalWarnings(warnings, map[int64]*EffectiveArrivalTime{
		1: {ArrivalTime: &clock, ClassException: &ClassArrivalExceptionInfo{
			SchoolClass: "4a", ArrivalTime: "12:45", Label: "Klasse 4a: Unterricht fällt aus",
		}},
		2: {ArrivalTime: &clock},
	}, inst)

	require.Len(t, warnings[1], 1)
	assert.Equal(t, "class_arrival_exception", warnings[1][0].Kind)
	assert.Equal(t, "Kommt heute um 12:45 Uhr (Klasse 4a: Unterricht fällt aus)", warnings[1][0].Message)
	require.NotNil(t, warnings[1][0].ExpectedArrival)
	assert.Equal(t, "12:45", *warnings[1][0].ExpectedArrival)
	assert.Empty(t, warnings[2], "a regular arrival at block start carries no line")
}

func mustClock(t *testing.T, hhmm string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", hhmm)
	require.NoError(t, err)
	return timezone.NormalizeWallClock(parsed)
}
