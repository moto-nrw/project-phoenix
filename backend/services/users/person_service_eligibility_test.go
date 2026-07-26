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

	filtered := filterStudentsEligibleOnDate(
		[]*userModels.Student{eligible, laterEnrolled, departed},
		date,
		timezone.TodayDate(),
	)

	require.Len(t, filtered, 1)
	assert.Same(t, eligible, filtered[0])
}
