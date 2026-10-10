package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachClassExceptionOnlyForProjectedClassRows(t *testing.T) {
	t.Parallel()

	clock := time.Date(1, 1, 1, 12, 45, 0, 0, time.UTC)

	effective := &careplan.EffectiveArrivalTime{ArrivalTime: &clock}
	attachClassException(effective, &careplan.ArrivalSchedule{
		Source:      careplan.ScheduleSourceClassException,
		SourceClass: "4a",
		SourceLabel: "Klasse 4a: Unterricht fällt aus",
	})
	require.NotNil(t, effective.ClassException)
	assert.Equal(t, "4a", effective.ClassException.SchoolClass)
	assert.Equal(t, "12:45", effective.ClassException.ArrivalTime)
	assert.Equal(t, "Klasse 4a: Unterricht fällt aus", effective.ClassException.Label)

	overridden := &careplan.EffectiveArrivalTime{ArrivalTime: &clock, IsException: true}
	attachClassException(overridden, &careplan.ArrivalSchedule{Source: careplan.ScheduleSourceClassException})
	assert.Nil(t, overridden.ClassException, "a per-child day exception hides the class one")

	regular := &careplan.EffectiveArrivalTime{ArrivalTime: &clock}
	attachClassException(regular, &careplan.ArrivalSchedule{Source: careplan.ScheduleSourceClassSchedule})
	assert.Nil(t, regular.ClassException)

	attachClassException(regular, nil)
	assert.Nil(t, regular.ClassException)
}

type weekendFridayArrivalBaseline struct{}

func (weekendFridayArrivalBaseline) Project(
	_ context.Context,
	studentIDs []int64,
	from, to calendar.Date,
) (*careplan.ArrivalBaselineProjection, error) {
	projection := &careplan.ArrivalBaselineProjection{
		WeeklyByStudentDate:  make(careplan.ArrivalPlansByStudent, len(studentIDs)),
		WeekendFollowsFriday: true,
	}
	for _, studentID := range studentIDs {
		weekly := careplan.ArrivalPlanByDate{}
		for date := from; !date.After(to); date = date.AddDays(1) {
			weekly[date] = careplan.ArrivalWeek{5: {
				StudentID: studentID,
				Weekday:   5,
			}}
		}
		projection.WeeklyByStudentDate[studentID] = weekly
	}
	return projection, nil
}

func TestProjectedWeekRangeReadsFridayRowsOnFollowingWeekend(t *testing.T) {
	t.Parallel()

	const studentID int64 = 42
	saturday := calendar.NewDate(2026, time.September, 12)
	service := &arrivalScheduleService{baselines: weekendFridayArrivalBaseline{}}

	rows, err := service.projectedWeekRange(context.Background(), []int64{studentID}, saturday, saturday.AddDays(1))
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, 5, rows[0].Weekday)
	assert.Equal(t, 5, rows[1].Weekday)
}
