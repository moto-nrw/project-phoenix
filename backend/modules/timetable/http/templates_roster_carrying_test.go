package timetablehttp

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
)

// withPickup sets the offering's pickup time on every weekday it books.
func withPickup(bookings []timetableModule.OfferingWeekdayBooking, pickup string) []timetableModule.OfferingWeekdayBooking {
	for index := range bookings {
		bookings[index].PickupTimes = map[int]string{}
		for _, weekday := range bookings[index].Weekdays {
			bookings[index].PickupTimes[weekday] = pickup
		}
	}
	return bookings
}

// carryingClassBookings: in class 1d Randstunde (until 08:00) books
// class[:4], Ganztag class[4], and every child of "Ganztag spät" also books
// the Randstunde. "Früh" is picked up at 07:30, before the Regeltermin
// (07:00 to 08:00) ends, and books class[5].
func carryingClassBookings(class []int64) []timetableModule.OfferingWeekdayBooking {
	bookings := withPickup(rosterReviewBookings(rosterReviewOfferingID, "Randstunde", "1d", class[:4], 1, 3), "08:00")
	bookings = append(bookings, withPickup(rosterReviewBookings(rosterReviewOfferingID+1, "Ganztag", "1d", class[4:5], 1, 3), "14:30")...)
	bookings = append(bookings, withPickup(rosterReviewBookings(rosterReviewOfferingID+2, "Ganztag spät", "1d", class[1:3], 1, 3), "16:00")...)
	return append(bookings, withPickup(rosterReviewBookings(rosterReviewOfferingID+3, "Früh", "1d", class[5:6], 1), "07:30")...)
}

var carryingClassOfferings = []templateRosterMaintenanceOfferingResponse{
	{ID: rosterReviewOfferingID, Name: "Randstunde"},
	{ID: rosterReviewOfferingID + 1, Name: "Ganztag"},
	{ID: rosterReviewOfferingID + 2, Name: "Ganztag spät"},
}

// TestTemplateRosterDeviationsTakeEveryCarryingOffering: the hint for a
// class Randstunde names every offering of the Klasse that carries it by its
// pickup time (#5342), also "Ganztag spät", whose children all book the
// Randstunde today; "Früh" ends before the Randstunde and stays out.
func TestTemplateRosterDeviationsTakeEveryCarryingOffering(t *testing.T) {
	t.Parallel()
	sources := &rosterMaintenanceSources{results: map[int64]timetableModule.TemplateRosterMaintenance{}}
	s := buildRosterReviewModule(t, sources)
	period := createTemplateTestPeriodRange(t, s.db, "Review-Period", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	class := createClassStudents(t, s.db, "1d", 6)
	sources.bookings = carryingClassBookings(class)
	template := s.createClassTemplate(t, "RDST 1d", "1d", period.ID, class)
	sources.results[template] = timetableModule.TemplateRosterMaintenance{Mode: "manual", DynamicTargetsManual: true}

	deviations := s.listDeviations(t, period.ID)

	require.Len(t, deviations, 1)
	got := deviations[0]
	assert.False(t, got.Ambiguous)
	assert.ElementsMatch(t, carryingClassOfferings, got.Offerings)
	assert.NotContains(t, got.Candidates, templateRosterMaintenanceOfferingResponse{ID: rosterReviewOfferingID + 3, Name: "Früh"})
	assert.Equal(t, 1, got.UnbookedCount, "class[5] books only Früh")
}

// TestDraftOfferingSuggestionTakesEveryCarryingOffering: the editor sends
// the planned end, so a new class Randstunde gets the same suggestion.
func TestDraftOfferingSuggestionTakesEveryCarryingOffering(t *testing.T) {
	t.Parallel()
	sources := &rosterMaintenanceSources{}
	s := buildRosterReviewModule(t, sources)
	router := s.draftRouter()
	period := createTemplateTestPeriodRange(t, s.db, "Draft-Period", rosterReviewMonday.AddDays(-30), rosterReviewMonday.AddDays(120), 1, nil)
	class := createClassStudents(t, s.db, "1d", 6)
	sources.bookings = carryingClassBookings(class)

	suggestion := getDraftSuggestion(t, router, url.Values{
		"period_id":      {fmt.Sprint(period.ID)},
		"type":           {timetableModule.GroupTypeCare},
		"list_kind":      {"edge_hours"},
		"school_classes": {"1d"},
		"end_time":       {"08:00"},
		"roster":         {draftRosterParam(1, class), draftRosterParam(3, class)},
	})

	require.NotNil(t, suggestion)
	assert.False(t, suggestion.Ambiguous)
	assert.ElementsMatch(t, carryingClassOfferings, suggestion.Offerings)
}
