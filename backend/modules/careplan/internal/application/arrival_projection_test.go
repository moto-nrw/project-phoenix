package application

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
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
