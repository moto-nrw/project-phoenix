package active

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/stretchr/testify/assert"
)

func TestVacationAbsenceHasWorkingDayBefore(t *testing.T) {
	cutoff := timezone.NewDate(2026, time.June, 8) // Monday
	yearStart := timezone.NewDate(2026, time.January, 1)

	assert.False(t, vacationAbsenceHasWorkingDayBefore(&activeModels.StaffAbsence{
		DateStart: cutoff.AddDays(-2), // Saturday
		DateEnd:   cutoff,
	}, yearStart, cutoff))
	assert.True(t, vacationAbsenceHasWorkingDayBefore(&activeModels.StaffAbsence{
		DateStart: cutoff.AddDays(-3), // Friday
		DateEnd:   cutoff,
	}, yearStart, cutoff))
}
