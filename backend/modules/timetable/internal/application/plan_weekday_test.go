package application

import (
	"testing"
	"time"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestRosterWeekdayUsesFridayOnlyWhenWeekendPlanIsEnabled(t *testing.T) {
	t.Parallel()

	friday := calendar.NewDate(2026, time.May, 8)
	assert.Equal(t, 5, planWeekday(friday, false))
	assert.Equal(t, 5, planWeekday(friday.AddDays(1), true))
	assert.Equal(t, 5, planWeekday(friday.AddDays(2), true))
	assert.Equal(t, 1, planWeekday(friday.AddDays(3), false))
	fridayOnly := 5
	assert.True(t, rosterWeekdayApplies(&fridayOnly, planWeekday(friday.AddDays(1), true)), "an enabled Saturday occurrence takes Friday's roster rows")
	assert.False(t, rosterWeekdayApplies(&fridayOnly, planWeekday(friday.AddDays(1), false)), "a retained Saturday occurrence keeps Saturday's roster rows")
	enrollment := &activityContract.StudentEnrollment{Weekday: &fridayOnly}
	assert.True(t, isEnrollmentValidOn(enrollment, friday.AddDays(1), 0, planWeekday(friday.AddDays(1), true)))
	assert.False(t, isEnrollmentValidOn(enrollment, friday.AddDays(1), 0, planWeekday(friday.AddDays(1), false)))
	assert.Equal(t, 6, planWeekday(friday.AddDays(1), false))
	assert.Equal(t, 5, planWeekday(friday.AddDays(1), true))
}
