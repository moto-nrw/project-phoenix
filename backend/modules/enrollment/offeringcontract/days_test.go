package offeringcontract

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDayCodesFollowISOWeekdays(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "mon", dayCode(1))
	assert.Equal(t, "sun", dayCode(7))
	assert.Empty(t, dayCode(0))
	assert.Empty(t, dayCode(8))
	assert.Equal(t, "fri", SchoolDayCode(5))
	assert.Empty(t, SchoolDayCode(6), "Saturday is no school day")
	assert.Equal(t, 3, DayNumber("wed"))
	assert.Zero(t, DayNumber("Wed"), "codes are lower case")
	assert.True(t, IsDayCode("sat"))
	assert.False(t, IsSchoolDayCode("sat"))
	assert.True(t, IsSchoolDayCode("mon"))
}

func TestSchoolDayCodesIsAFreshSlice(t *testing.T) {
	t.Parallel()
	days := SchoolDayCodes()
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, days)
	days[0] = "sun"
	assert.Equal(t, "mon", SchoolDayCodes()[0])
}
