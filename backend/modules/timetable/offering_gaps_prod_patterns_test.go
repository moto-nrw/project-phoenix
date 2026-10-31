package timetable

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The production comparison of #5343 replayed on the real implementation,
// anonymised to classes, clock times, weekday sets and counts.

// schoolABookings are the Randstunde bookings of school A: per class, how
// many children book which weekdays.
var schoolABookings = map[string]map[string]int{
	"1a": {"5": 2, "12345": 10, "134": 1, "245": 1},
	"1b": {"1234": 1, "12345": 19, "135": 2},
	"1c": {"5": 1, "1": 1, "15": 2, "14": 1, "1234": 2, "12345": 11},
	"2a": {"5": 2, "145": 1, "12345": 14, "1345": 1, "2345": 1},
	"2b": {"5": 1, "12345": 15, "25": 1, "2345": 2},
	"2c": {"5": 1, "1235": 1, "1234": 1, "12345": 11, "135": 1, "245": 2},
	"3a": {"5": 2, "1234": 1, "12345": 4, "2345": 1},
	"3b": {"5": 3, "15": 1, "123": 1, "12345": 5, "45": 1},
	"3c": {"5": 2, "1245": 1, "12345": 7, "45": 1},
	"4a": {"5": 3, "1234": 1, "12345": 1},
	"4b": {"123": 1, "12345": 2, "135": 1, "134": 1, "45": 1},
	"4c": {"5": 3, "12345": 1, "1345": 1},
}

// schoolAKlassentag: when each class arrives, Monday to Friday.
var schoolAKlassentag = map[string][5]int{
	"1a": {clock1145, clock1245, clock1145, clock1145, clock1245},
	"1b": {clock1145, clock1145, clock1245, clock1245, clock1145},
	"1c": {clock1245, clock1145, clock1145, clock1245, clock1145},
	"2a": {clock1145, clock1245, clock1245, clock1245, clock1145},
	"2b": {clock1245, clock1245, clock1145, clock1245, clock1145},
	"2c": {clock1245, clock1145, clock1245, clock1245, clock1145},
	"3a": {clock1245, clock1330, clock1245, clock1245, clock1245},
	"3b": {clock1245, clock1245, clock1245, clock1330, clock1245},
	"3c": {clock1245, clock1245, clock1245, clock1330, clock1245},
	"4a": {clock1245, clock1245, clock1330, clock1245, clock1245},
	"4b": {clock1245, clock1245, clock1330, clock1245, clock1245},
	"4c": {clock1245, clock1245, clock1330, clock1245, clock1245},
}

// schoolAKlassenRandstunden: the weekdays of each class's Klassen-
// Randstunde (11:45–12:45), exactly the days the class arrives at 11:45.
var schoolAKlassenRandstunden = map[string][]int{
	"1a": {1, 3, 4}, "1b": {1, 2, 5}, "1c": {2, 3, 5}, "2a": {1, 5}, "2b": {3, 5}, "2c": {2, 5},
}

type schoolAReplay struct {
	bookings  []OfferingWeekdayBooking
	templates []OfferingGapTemplate
	arrivals  OfferingGapArrivals
}

func replaySchoolA(t *testing.T) schoolAReplay {
	t.Helper()
	replay := schoolAReplay{arrivals: weeklyArrivals(map[string]map[int]int{})}
	members := map[string][]int64{}
	next := int64(1000)
	for class, sets := range schoolABookings {
		for days, count := range sets {
			weekdays := make([]int, 0, len(days))
			for _, day := range strings.Split(days, "") {
				weekdays = append(weekdays, int(day[0]-'0'))
			}
			replay.bookings = append(replay.bookings, classDayBookings(class, next, count, weekdays...)...)
			members[class] = append(members[class], studentRange(next, count)...)
			next += int64(count)
		}
	}
	for class, times := range schoolAKlassentag {
		replay.arrivals.Weekly[class] = map[int]int{1: times[0], 2: times[1], 3: times[2], 4: times[3], 5: times[4]}
	}
	for class, weekdays := range schoolAKlassenRandstunden {
		replay.templates = append(replay.templates, classTemplate(members[class], clock1145, clock1245, weekdays...))
	}
	require.Len(t, replay.bookings, 155)
	return replay
}

func gapStudents(gaps []OfferingGap) int {
	total := 0
	for _, gap := range gaps {
		total += gap.StudentCount
	}
	return total
}

// TestDetectOfferingGapsSchoolAReplay: #5140 named all 155 children; with
// the Klassentag and the pickup time none is left, because Jahrgang 1/2 sits
// in its Klassen-Randstunde on every day its class arrives before the pickup
// and Jahrgang 3/4 never does. Without pickup times the strict day rule
// still names 149: only six children book nothing but their class's
// Randstunde days.
func TestDetectOfferingGapsSchoolAReplay(t *testing.T) {
	t.Parallel()
	replay := replaySchoolA(t)

	sourceCheckOnly := make([]OfferingWeekdayBooking, len(replay.bookings))
	copy(sourceCheckOnly, replay.bookings)
	for i := range sourceCheckOnly {
		sourceCheckOnly[i].ClassDayApplies = false
	}
	unread := make([]OfferingGapTemplate, len(replay.templates))
	for i, template := range replay.templates {
		template.Roster = nil
		unread[i] = template
	}
	assert.Equal(t, 155, gapStudents(DetectOfferingGaps(withArrivals(replay.arrivals), unread, sourceCheckOnly)), "the rule of #5140")

	assert.Empty(t, DetectOfferingGaps(withArrivals(replay.arrivals), replay.templates, replay.bookings), "#5343 with Klassentag and pickup time")

	withoutPickup := make([]OfferingWeekdayBooking, len(replay.bookings))
	copy(withoutPickup, replay.bookings)
	for i := range withoutPickup {
		withoutPickup[i].PickupTimes = nil
	}
	assert.Equal(t, 149, gapStudents(DetectOfferingGaps(withArrivals(replay.arrivals), replay.templates, withoutPickup)), "strict day rule without pickup times")
}

// TestDetectOfferingGapsHandListPatterns are the two schools without pickup
// times: one plans every booked child on hand-kept care lists every day,
// one keeps hand lists without the booked children.
func TestDetectOfferingGapsHandListPatterns(t *testing.T) {
	t.Parallel()
	children := studentRange(800, 6)
	handLists := []OfferingGapTemplate{
		{Occurrences: gapRuns(1, 2, 3, 4), Times: clockTimes(13*60, 16*60, 1, 2, 3, 4), Kind: RosterOfferingCare, Roster: &TemplatePlannedRoster{Entries: entriesFor(children)}},
		{Occurrences: gapRuns(5), Times: clockTimes(13*60, 16*60, 5), Kind: RosterOfferingCare, Roster: &TemplatePlannedRoster{Entries: entriesFor(children)}},
	}
	bookings := gapBookings(gapGanztag, "Ganztag", "2a", 800, 6, 1, 2, 3, 4, 5)
	assert.Empty(t, DetectOfferingGaps(withArrivals(schoolAArrivals()), handLists, bookings), "hand-kept care lists every day plan the children")

	otherChildren := []OfferingGapTemplate{
		{Occurrences: gapRuns(1, 2, 3, 4, 5), Times: clockTimes(13*60, 16*60, 1, 2, 3, 4, 5), Kind: RosterOfferingCare, Roster: &TemplatePlannedRoster{Entries: entriesFor(studentRange(900, 6))}},
		{Occurrences: gapRuns(1, 2, 3, 4, 5), Times: clockTimes(12*60, 12*60+30, 1, 2, 3, 4, 5), Kind: RosterOfferingLunch, Roster: &TemplatePlannedRoster{Entries: entriesFor(children)}},
	}
	gaps := DetectOfferingGaps(gapScope, otherChildren, bookings)
	require.Len(t, gaps, 1, "a hand list without the children and a Mensa list change nothing")
	assert.Equal(t, 6, gaps[0].StudentCount)
	assert.Equal(t, []int{1, 2, 3, 4, 5}, gaps[0].Weekdays)
}
