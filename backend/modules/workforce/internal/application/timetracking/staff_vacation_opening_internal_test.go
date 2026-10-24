package timetracking

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestVacationAbsenceHasWorkingDayBefore(t *testing.T) {
	t.Parallel()

	cutoff := NewDate(2026, time.June, 8) // Monday
	yearStart := NewDate(2026, time.January, 1)

	assert.False(t, vacationAbsenceHasWorkingDayBefore(&StaffAbsence{
		DateStart: cutoff.AddDays(-2), // Saturday
		DateEnd:   cutoff,
	}, yearStart, cutoff))
	assert.True(t, vacationAbsenceHasWorkingDayBefore(&StaffAbsence{
		DateStart: cutoff.AddDays(-3), // Friday
		DateEnd:   cutoff,
	}, yearStart, cutoff))
}
