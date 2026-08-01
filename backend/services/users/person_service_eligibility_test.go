package users

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterStudentsEligibleOnDate_UsesEnrollmentWindowForPastDates(t *testing.T) {
	date := timezone.TodayDate().AddDays(-1)
	fromBeforeDate := date.AddDays(-1)
	fromToday := timezone.TodayDate()
	untilBeforeDate := date.AddDays(-1)

	eligible := &userModels.Student{EnrolledFrom: &fromBeforeDate}
	laterEnrolled := &userModels.Student{EnrolledFrom: &fromToday}
	departed := &userModels.Student{EnrolledUntil: &untilBeforeDate}

	filtered := filterStudentsEligibleOnDate([]*userModels.Student{eligible, laterEnrolled, departed}, date, timezone.TodayDate())

	require.Len(t, filtered, 1)
	assert.Same(t, eligible, filtered[0])
}

func TestFilterStudentsEligibleOnDate_IncludesImmediatelyActiveFutureStudentToday(t *testing.T) {
	today := timezone.TodayDate()
	tomorrow := today.AddDays(1)

	filtered := filterStudentsEligibleOnDate([]*userModels.Student{
		{Status: userModels.StudentStatusActive, EnrolledFrom: &tomorrow},
		{Status: userModels.StudentStatusActive, EnrolledFrom: &today},
	}, today, today)

	require.Len(t, filtered, 2)
	assert.Equal(t, tomorrow, *filtered[0].EnrolledFrom)
	assert.Equal(t, today, *filtered[1].EnrolledFrom)
}

// Immediate activation lifts the enrolled_from bound from today onward only —
// the same boundary slotlists.eligibleOn applies (#1565). A past day must keep
// the bound, and a non-active child never gets the override.
func TestFilterStudentsEligibleOnDate_ImmediateActivationOnlyFromTodayOnward(t *testing.T) {
	today := timezone.TodayDate()
	yesterday := today.AddDays(-1)
	nextWeek := today.AddDays(7)

	activeFuture := &userModels.Student{Status: userModels.StudentStatusActive, EnrolledFrom: &nextWeek}
	pendingFuture := &userModels.Student{Status: userModels.StudentStatusPending, EnrolledFrom: &nextWeek}

	assert.Empty(t, filterStudentsEligibleOnDate([]*userModels.Student{activeFuture}, yesterday, today),
		"an active child is not retroactively enrolled before enrolled_from")
	assert.Empty(t, filterStudentsEligibleOnDate([]*userModels.Student{pendingFuture}, today, today),
		"only an active status gets the immediate-activation override")
	assert.Len(t, filterStudentsEligibleOnDate([]*userModels.Student{activeFuture}, nextWeek, today), 1,
		"the enrollment window itself still governs future dates")
}
