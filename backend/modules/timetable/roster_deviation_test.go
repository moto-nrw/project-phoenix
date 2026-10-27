package timetable_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	randstundeID = int64(301)
	ganztagID    = int64(302)
)

// classRoster plans the given children on every weekday of the template.
func classRoster(weekdays []int, students []int64, classes ...string) timetable.TemplateFutureRoster {
	roster := timetable.TemplateFutureRoster{Weekdays: map[int][]int64{}, TargetSchoolClasses: classes}
	for _, weekday := range weekdays {
		roster.Weekdays[weekday] = students
	}
	return roster
}

func booking(offeringID int64, name string, studentID int64, class string, weekdays ...int) timetable.OfferingWeekdayBooking {
	return timetable.OfferingWeekdayBooking{
		OfferingID: offeringID, OfferingName: name, StudentID: studentID, SchoolClass: class, Weekdays: weekdays,
	}
}

func studentRange(first int64, count int) []int64 {
	ids := make([]int64, 0, count)
	for i := range count {
		ids = append(ids, first+int64(i))
	}
	return ids
}

// TestDetectRosterDeviationSchuleAmBerg: the whole class 1a is planned by
// hand on every weekday; "Randstunde" books part of the class on some days.
// The hint names Randstunde with the Klassenfilter and counts the children
// planned without a booking.
func TestDetectRosterDeviationSchuleAmBerg(t *testing.T) {
	t.Parallel()
	class := studentRange(1001, 20)
	roster := classRoster([]int{1, 2, 3, 4, 5}, class, "Klasse 1a")
	bookings := []timetable.OfferingWeekdayBooking{}
	// Twelve children book Randstunde Monday to Friday.
	for _, studentID := range class[:12] {
		bookings = append(bookings, booking(randstundeID, "Randstunde", studentID, "Klasse 1a", 1, 2, 3, 4, 5))
	}
	// Two book only Monday; one of them is not on the roster at all.
	bookings = append(bookings,
		booking(randstundeID, "Randstunde", class[12], "klasse 1a", 1),
		booking(randstundeID, "Randstunde", 2001, "Klasse 1a", 1),
		// A child of another class does not count for this Regeltermin.
		booking(randstundeID, "Randstunde", 3001, "Klasse 2b", 1, 2, 3, 4, 5),
	)

	deviation := timetable.DetectRosterDeviation(roster, bookings)

	require.NotNil(t, deviation)
	assert.Equal(t, timetable.OfferingRef{ID: randstundeID, Name: "Randstunde"}, deviation.Offering)
	assert.Equal(t, []string{"Klasse 1a"}, deviation.SchoolClasses)
	assert.Empty(t, deviation.GradeLevels)
	// class[12] is planned Tuesday to Friday without a booking; class[13:]
	// has no booking at all: 1 + 7 children.
	assert.Equal(t, 8, deviation.UnbookedCount)
	assert.Equal(t, 1, deviation.MissingCount, "child 2001 is booked on Monday but not planned")
	assert.Equal(t, []timetable.OfferingRef{{ID: randstundeID, Name: "Randstunde"}}, deviation.Candidates)
}

func TestDetectRosterDeviationThresholds(t *testing.T) {
	t.Parallel()
	weekdays := []int{1, 2, 3, 4, 5}

	t.Run("fewer than three matched children", func(t *testing.T) {
		t.Parallel()
		roster := classRoster(weekdays, studentRange(1001, 10), "Klasse 1a")
		bookings := []timetable.OfferingWeekdayBooking{
			booking(randstundeID, "Randstunde", 1001, "Klasse 1a", 1, 2, 3, 4, 5),
			booking(randstundeID, "Randstunde", 1002, "Klasse 1a", 1, 2, 3, 4, 5),
		}
		assert.Nil(t, timetable.DetectRosterDeviation(roster, bookings))
	})

	t.Run("below 80 percent coverage", func(t *testing.T) {
		t.Parallel()
		roster := classRoster(weekdays, studentRange(1001, 10), "Klasse 1a")
		bookings := []timetable.OfferingWeekdayBooking{}
		for _, studentID := range studentRange(1001, 7) {
			bookings = append(bookings, booking(randstundeID, "Randstunde", studentID, "Klasse 1a", 1, 2, 3, 4, 5))
		}
		// Two of nine booked children are not planned: 7/9 < 80 %.
		for _, studentID := range studentRange(4001, 2) {
			bookings = append(bookings, booking(randstundeID, "Randstunde", studentID, "Klasse 1a", 1, 2, 3, 4, 5))
		}
		assert.Nil(t, timetable.DetectRosterDeviation(roster, bookings))
	})

	t.Run("exactly 80 percent and three children pass", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{1}, studentRange(1001, 6))
		bookings := []timetable.OfferingWeekdayBooking{}
		for _, studentID := range studentRange(1001, 4) {
			bookings = append(bookings, booking(randstundeID, "Randstunde", studentID, "", 1))
		}
		bookings = append(bookings, booking(randstundeID, "Randstunde", 4001, "", 1))
		deviation := timetable.DetectRosterDeviation(roster, bookings)
		require.NotNil(t, deviation)
		assert.Equal(t, 2, deviation.UnbookedCount)
		assert.Equal(t, 1, deviation.MissingCount)
	})

	t.Run("an exact match is no deviation", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{2}, studentRange(1001, 4), "Klasse 1a")
		bookings := []timetable.OfferingWeekdayBooking{}
		for _, studentID := range studentRange(1001, 4) {
			bookings = append(bookings, booking(randstundeID, "Randstunde", studentID, "Klasse 1a", 2, 4))
		}
		assert.Nil(t, timetable.DetectRosterDeviation(roster, bookings),
			"bookings on weekdays the Regeltermin does not run do not count")
	})
}

// TestDetectRosterDeviationPrefersClosestCandidate: two offerings cover the
// roster fully; the one closer to the planned children wins, both are named.
func TestDetectRosterDeviationPrefersClosestCandidate(t *testing.T) {
	t.Parallel()
	roster := classRoster([]int{1, 2}, studentRange(1001, 10))
	roster.TargetGradeLevels = []int{1}
	grade := 1
	bookings := []timetable.OfferingWeekdayBooking{}
	for _, studentID := range studentRange(1001, 4) {
		entry := booking(randstundeID, "Randstunde", studentID, "1a", 1, 2)
		entry.GradeLevel = &grade
		bookings = append(bookings, entry)
	}
	for _, studentID := range studentRange(1001, 8) {
		entry := booking(ganztagID, "Ganztag", studentID, "1a", 1, 2)
		entry.GradeLevel = &grade
		bookings = append(bookings, entry)
	}
	// A child without grade data is outside the Jahrgang target.
	bookings = append(bookings, booking(ganztagID, "Ganztag", 5001, "", 1, 2))

	deviation := timetable.DetectRosterDeviation(roster, bookings)

	require.NotNil(t, deviation)
	assert.Equal(t, ganztagID, deviation.Offering.ID)
	assert.Equal(t, []int{1}, deviation.GradeLevels)
	assert.Equal(t, 2, deviation.UnbookedCount)
	assert.Zero(t, deviation.MissingCount)
	assert.Equal(t, []timetable.OfferingRef{{ID: ganztagID, Name: "Ganztag"}, {ID: randstundeID, Name: "Randstunde"}}, deviation.Candidates)
}

func TestPreviewOfferingSource(t *testing.T) {
	t.Parallel()
	roster := classRoster([]int{1, 3}, studentRange(1001, 4), "Klasse 1a")
	bookings := []timetable.OfferingWeekdayBooking{
		booking(randstundeID, "Randstunde", 1001, "Klasse 1a", 1, 3),
		booking(randstundeID, "Randstunde", 1002, "klasse 1a", 1),
		booking(randstundeID, "Randstunde", 2001, "Klasse 1a", 3),
		booking(randstundeID, "Randstunde", 3001, "Klasse 2b", 1, 3),
		booking(ganztagID, "Ganztag", 1003, "Klasse 1a", 1, 3),
	}

	changes := timetable.PreviewOfferingSource(roster, timetable.OfferingSourceFilter{
		OfferingIDs: []int64{randstundeID}, SchoolClasses: []string{"Klasse 1a"},
	}, bookings)

	assert.Equal(t, []timetable.OfferingSourceWeekdayChange{
		{Weekday: 1, Added: []int64{}, Removed: []int64{1003, 1004}, KeptCount: 2},
		{Weekday: 3, Added: []int64{2001}, Removed: []int64{1002, 1003, 1004}, KeptCount: 1},
	}, changes)
}

// TestDetectRosterDeviationOfferingKind: a Randstunde holding the whole class
// matches every offering of the class. The lunch must not explain a care
// block, and a Mensa list takes the lunch offering.
func TestDetectRosterDeviationOfferingKind(t *testing.T) {
	t.Parallel()
	const lunchID = int64(303)
	class := studentRange(1001, 20)
	bookings := []timetable.OfferingWeekdayBooking{}
	for _, studentID := range class {
		lunch := booking(lunchID, "Mittagessen", studentID, "Klasse 1b", 1, 2, 3, 4, 5)
		lunch.IncludesLunch = true
		bookings = append(bookings, lunch)
	}
	for _, studentID := range class[:6] {
		care := booking(randstundeID, "Kurzbetreuung", studentID, "Klasse 1b", 1, 2, 3, 4, 5)
		care.CountsAsCare = true
		bookings = append(bookings, care)
	}
	mensa := "mensa"
	edgeHours := "edge_hours"

	t.Run("care block ignores the lunch", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{1, 2, 3, 4, 5}, class, "Klasse 1b")
		roster.OfferingKind = timetable.RosterOfferingKindFor(timetable.GroupTypeCare, &edgeHours)

		deviation := timetable.DetectRosterDeviation(roster, bookings)

		require.NotNil(t, deviation)
		assert.Equal(t, randstundeID, deviation.Offering.ID)
		assert.Equal(t, 14, deviation.UnbookedCount)
		assert.Len(t, deviation.Candidates, 1)
	})

	t.Run("mensa list takes the lunch", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{1, 2, 3, 4, 5}, append(class[:18:18], 9001, 9002), "Klasse 1b")
		roster.OfferingKind = timetable.RosterOfferingKindFor(timetable.GroupTypeCare, &mensa)

		deviation := timetable.DetectRosterDeviation(roster, bookings)

		require.NotNil(t, deviation)
		assert.Equal(t, lunchID, deviation.Offering.ID)
		assert.Equal(t, 2, deviation.MissingCount)
	})

	t.Run("kind of an AG and an external block", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, timetable.RosterOfferingCourse, timetable.RosterOfferingKindFor(timetable.GroupTypeActivity, nil))
		assert.Equal(t, timetable.RosterOfferingAny, timetable.RosterOfferingKindFor(timetable.GroupTypeExternal, nil))
	})
}

// TestSuggestOfferingSource: a Regeltermin being planned (#5104) gets the
// offering even when its children match the bookings exactly; the hint of
// an existing one stays silent then. Below the rule both stay silent.
func TestSuggestOfferingSource(t *testing.T) {
	t.Parallel()
	class := studentRange(1001, 4)
	roster := classRoster([]int{1, 3}, class, "Klasse 1b")
	bookings := []timetable.OfferingWeekdayBooking{}
	for _, studentID := range class {
		bookings = append(bookings, booking(randstundeID, "Randstunde", studentID, "Klasse 1b", 1, 3))
	}

	assert.Nil(t, timetable.DetectRosterDeviation(roster, bookings), "an exact match is no deviation")
	suggestion := timetable.SuggestOfferingSource(roster, bookings)
	require.NotNil(t, suggestion)
	assert.Equal(t, timetable.OfferingRef{ID: randstundeID, Name: "Randstunde"}, suggestion.Offering)
	assert.Equal(t, []string{"Klasse 1b"}, suggestion.SchoolClasses)
	assert.Zero(t, suggestion.UnbookedCount)
	assert.Zero(t, suggestion.MissingCount)

	small := classRoster([]int{1, 3}, class[:2], "Klasse 1b")
	assert.Nil(t, timetable.SuggestOfferingSource(small, bookings[:2]), "fewer than three children")
}
