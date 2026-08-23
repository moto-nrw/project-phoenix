package schedule

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

func TestEffectiveArrivalRowKeepsBookingCareDayWithoutTime(t *testing.T) {
	t.Parallel()

	row := effectiveArrivalRow(42, scheduleModel.WeekdayMonday, nil, nil)

	require.NotNil(t, row)
	assert.Equal(t, int64(42), row.StudentID)
	assert.Equal(t, scheduleModel.WeekdayMonday, row.Weekday)
	assert.True(t, row.ExpectedArrival.IsZero())
	assert.Empty(t, row.Source)
}

func TestEffectiveArrivalRowDoesNotInventClassSourceWithoutClassTime(t *testing.T) {
	t.Parallel()

	stored := &scheduleModel.StudentArrivalSchedule{
		StudentID: 42,
		Weekday:   scheduleModel.WeekdayMonday,
	}
	row := effectiveArrivalRow(42, scheduleModel.WeekdayMonday, stored, nil)

	require.NotNil(t, row)
	assert.Empty(t, row.Source)
	assert.Empty(t, row.SourceClass)
}
