package users

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterStudentsStartedOnDate_UsesEnrollmentStartForPastDates(t *testing.T) {
	t.Parallel()

	date := timezone.TodayDate().AddDays(-1)
	fromBeforeDate := date.AddDays(-1)
	fromToday := timezone.TodayDate()
	untilBeforeDate := date.AddDays(-1)

	eligible := &userModels.Student{EnrolledFrom: &fromBeforeDate}
	laterEnrolled := &userModels.Student{EnrolledFrom: &fromToday}
	departed := &userModels.Student{EnrolledUntil: &untilBeforeDate}

	filtered := filterStudentsStartedOnDate([]*userModels.Student{eligible, laterEnrolled, departed}, date, timezone.TodayDate())

	require.Len(t, filtered, 2)
	assert.Same(t, eligible, filtered[0])
	assert.Same(t, departed, filtered[1])
}

func TestFilterStudentsEligibleOnDate_IncludesImmediatelyActiveFutureStudentToday(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2026, 8, 24)
	tomorrow := today.AddDays(1)

	filtered := filterStudentsStartedOnDate([]*userModels.Student{
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
	t.Parallel()

	today := timezone.TodayDate()
	yesterday := today.AddDays(-1)
	nextWeek := today.AddDays(7)

	activeFuture := &userModels.Student{Status: userModels.StudentStatusActive, EnrolledFrom: &nextWeek}
	pendingFuture := &userModels.Student{Status: userModels.StudentStatusPending, EnrolledFrom: &nextWeek}

	assert.Empty(t, filterStudentsStartedOnDate([]*userModels.Student{activeFuture}, yesterday, today),
		"an active child is not retroactively enrolled before enrolled_from")
	assert.Empty(t, filterStudentsStartedOnDate([]*userModels.Student{pendingFuture}, today, today),
		"only an active status gets the immediate-activation override")
	assert.Len(t, filterStudentsStartedOnDate([]*userModels.Student{activeFuture}, nextWeek, today), 1,
		"the enrollment window itself still governs future dates")
}

func TestFilterStudentsStartedOnDate_SkipsNilStudents(t *testing.T) {
	t.Parallel()

	today := timezone.TodayDate()
	kept := &userModels.Student{Status: userModels.StudentStatusActive}

	filtered := filterStudentsStartedOnDate([]*userModels.Student{nil, kept, nil}, today, today)

	require.Len(t, filtered, 1)
	assert.Same(t, kept, filtered[0])
}

func TestFilterStudentsStartedOnDate_ExcludesLegacyInactiveStudentWithoutEnrollmentBounds(t *testing.T) {
	t.Parallel()

	inactive := &userModels.Student{Status: userModels.StudentStatusInactive}
	assert.Empty(t, filterStudentsStartedOnDate([]*userModels.Student{inactive}, timezone.TodayDate(), timezone.TodayDate()))
}
