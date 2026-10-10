package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

// statusInactive is the legacy lifecycle status of a child without enrollment
// dates; the public directory names no constant for it.
const statusInactive = "inactive"

func TestStudentStartedOnDate_UsesEnrollmentStartForPastDates(t *testing.T) {
	t.Parallel()

	date := calendar.TodayDate().AddDays(-1)
	fromBeforeDate := date.AddDays(-1)
	fromToday := calendar.TodayDate()
	untilBeforeDate := date.AddDays(-1)

	assert.True(t, studentStartedOnDate(&fromBeforeDate, nil, "", date, calendar.TodayDate()),
		"enrolled before the date")
	assert.False(t, studentStartedOnDate(&fromToday, nil, "", date, calendar.TodayDate()),
		"enrolled only after the date")
	assert.True(t, studentStartedOnDate(nil, &untilBeforeDate, "", date, calendar.TodayDate()),
		"the upper bound is the participation resolver's, not this rule's")
}

func TestStudentStartedOnDate_IncludesImmediatelyActiveFutureStudentToday(t *testing.T) {
	t.Parallel()

	today := calendar.NewDate(2026, 8, 24)
	tomorrow := today.AddDays(1)

	assert.True(t, studentStartedOnDate(&tomorrow, nil, peopledirectory.StudentStatusActive, today, today))
	assert.True(t, studentStartedOnDate(&today, nil, peopledirectory.StudentStatusActive, today, today))
}

// Immediate activation lifts the enrolled_from bound from today onward only —
// the same boundary slotlists.eligibleOn applies (#1565). A past day must keep
// the bound, and a non-active child never gets the override.
func TestStudentStartedOnDate_ImmediateActivationOnlyFromTodayOnward(t *testing.T) {
	t.Parallel()

	today := calendar.TodayDate()
	yesterday := today.AddDays(-1)
	nextWeek := today.AddDays(7)

	assert.False(t, studentStartedOnDate(&nextWeek, nil, peopledirectory.StudentStatusActive, yesterday, today),
		"an active child is not retroactively enrolled before enrolled_from")
	assert.False(t, studentStartedOnDate(&nextWeek, nil, peopledirectory.StudentStatusPending, today, today),
		"only an active status gets the immediate-activation override")
	assert.True(t, studentStartedOnDate(&nextWeek, nil, peopledirectory.StudentStatusActive, nextWeek, today),
		"the enrollment window itself still governs future dates")
}

func TestStudentStartedOnDate_ExcludesLegacyInactiveStudentWithoutEnrollmentBounds(t *testing.T) {
	t.Parallel()

	assert.False(t, studentStartedOnDate(nil, nil, statusInactive, calendar.TodayDate(), calendar.TodayDate()))
}
