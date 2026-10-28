package timetablehttp

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestOfferingGapsSeriesEndingBeforeTheBooking: a Regeltermin whose series
// ends after three weeks does not cover the Mondays the booking keeps after
// that. Coverage is a matter of dates, not only of the weekday (#5140).
func TestOfferingGapsSeriesEndingBeforeTheBooking(t *testing.T) {
	t.Parallel()
	s := buildOfferingGapsModule(t)
	period := createTemplateTestPeriodRange(t, s.db, "Gap-Series-End", rosterReviewMonday, rosterReviewMonday.AddDays(120), 1, nil)
	s.sources.bookings = withGrade(rosterReviewBookings(rosterReviewOfferingID, "Randstunde", "3a", createClassStudents(t, s.db, "3a", 2), 1), 3)
	s.createSourcedTemplate(t, "Randstunde März", period.ID, rosterReviewOfferingID, []int{3}, rosterReviewMonday.AddDays(20))

	gaps := s.listGaps(t, period.ID)

	require.Len(t, gaps, 1, "the Mondays after the series end have no Regeltermin")
	assert.Equal(t, []int{timetableModule.WeekdayMonday}, gaps[0].Weekdays)
	assert.Equal(t, 2, gaps[0].StudentCount)
}

// TestOfferingGapsIgnoreInactivePeriods: only an active planning period is
// compared. An inactive one shows no hint, and a decision for it is refused
// and stored nowhere, so it cannot hide a real hint later.
func TestOfferingGapsIgnoreInactivePeriods(t *testing.T) {
	t.Parallel()
	s := buildOfferingGapsModule(t)
	period := createTemplateTestPeriodRange(t, s.db, "Gap-Inactive", rosterReviewMonday, rosterReviewMonday.AddDays(120), 1, nil)
	_, err := s.db.NewUpdate().TableExpr("schedule.calendar_periods").Set("is_active = false").
		Where("id = ?", period.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	s.sources.bookings = withGrade(rosterReviewBookings(rosterReviewOfferingID, "Randstunde", "3a", createClassStudents(t, s.db, "3a", 2), 1), 3)

	assert.Empty(t, s.listGaps(t, period.ID), "an inactive planning period shows no hint")
	w := s.decide(t, rosterReviewOfferingID, period.ID, true)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Empty(t, s.sources.decisions, "nothing is stored for an inactive period")

	w = s.decide(t, rosterReviewOfferingID, period.ID+1_000_000, true)
	assert.Equal(t, http.StatusBadRequest, w.Code, "an unknown period is refused: body=%s", w.Body.String())
}

// TestOfferingGapsDecisionHoldsForItsPeriod: "Kein Regeltermin nötig" in one
// planning period leaves the hint of another period that the offering fits
// open, so a decision never hides a later real hint elsewhere.
func TestOfferingGapsDecisionHoldsForItsPeriod(t *testing.T) {
	t.Parallel()
	s := buildOfferingGapsModule(t)
	first := createTemplateTestPeriodRange(t, s.db, "Gap-First", rosterReviewMonday, rosterReviewMonday.AddDays(60), 1, nil)
	second := createTemplateTestPeriodRange(t, s.db, "Gap-Second", rosterReviewMonday.AddDays(61), rosterReviewMonday.AddDays(120), 1, nil)
	s.sources.bookings = withGrade(rosterReviewBookings(rosterReviewOfferingID, "Randstunde", "3a", createClassStudents(t, s.db, "3a", 2), 1), 3)

	w := s.decide(t, rosterReviewOfferingID, first.ID, true)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	firstGaps := s.listGaps(t, first.ID)
	require.Len(t, firstGaps, 1)
	assert.True(t, firstGaps[0].Acknowledged, "the decision hides the hint of its period")
	secondGaps := s.listGaps(t, second.ID)
	require.Len(t, secondGaps, 1)
	assert.False(t, secondGaps[0].Acknowledged, "the other period keeps its hint")
	assert.Empty(t, secondGaps[0].AcknowledgedClasses)
}
