package timetable_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	randstundeID = int64(301)
	ganztagID    = int64(302)
	spaetID      = int64(303)
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
	assert.Equal(t, []timetable.OfferingRef{{ID: randstundeID, Name: "Randstunde"}}, deviation.Offerings)
	assert.Equal(t, []string{"Klasse 1a"}, deviation.SchoolClasses)
	assert.Empty(t, deviation.GradeLevels)
	// class[12] is planned Tuesday to Friday without a booking; class[13:]
	// has no booking at all: 1 + 7 children.
	assert.Equal(t, 8, deviation.UnbookedCount)
	assert.Equal(t, 1, deviation.MissingCount, "child 2001 is booked on Monday but not planned")
	// The hint names each child with its weekdays (#5103 review): class[12]
	// lacks a booking Tuesday to Friday only.
	require.Len(t, deviation.Unbooked, 8)
	assert.Equal(t, timetable.RosterDeviationChild{StudentID: class[12], Weekdays: []int{2, 3, 4, 5}}, deviation.Unbooked[0])
	assert.Equal(t, timetable.RosterDeviationChild{StudentID: class[13], Weekdays: []int{1, 2, 3, 4, 5}}, deviation.Unbooked[1])
	assert.Equal(t, []timetable.RosterDeviationChild{{StudentID: 2001, Weekdays: []int{1}}}, deviation.Missing)
	assert.Equal(t, []timetable.OfferingRef{{ID: randstundeID, Name: "Randstunde"}}, deviation.Candidates)
	assert.False(t, deviation.Ambiguous, "one candidate is a clear suggestion")
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
// Every Randstunde child also books Ganztag, so Randstunde explains no child
// Ganztag does not: it is no equal alternative and does not join (#5139).
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
	assert.Equal(t, []timetable.OfferingRef{{ID: ganztagID, Name: "Ganztag"}}, deviation.Offerings)
	assert.Equal(t, []int{1}, deviation.GradeLevels)
	assert.Equal(t, 2, deviation.UnbookedCount)
	assert.Zero(t, deviation.MissingCount)
	assert.Equal(t, []timetable.OfferingRef{{ID: ganztagID, Name: "Ganztag"}, {ID: randstundeID, Name: "Randstunde"}}, deviation.Candidates)
	assert.False(t, deviation.Ambiguous, "Randstunde cannot replace Ganztag without losing four children")
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
		assert.Equal(t, []timetable.OfferingRef{{ID: randstundeID, Name: "Kurzbetreuung"}}, deviation.Offerings)
		assert.Equal(t, 14, deviation.UnbookedCount)
		assert.Len(t, deviation.Candidates, 1)
	})

	t.Run("mensa list takes the lunch", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{1, 2, 3, 4, 5}, append(class[:18:18], 9001, 9002), "Klasse 1b")
		roster.OfferingKind = timetable.RosterOfferingKindFor(timetable.GroupTypeCare, &mensa)

		deviation := timetable.DetectRosterDeviation(roster, bookings)

		require.NotNil(t, deviation)
		assert.Equal(t, []timetable.OfferingRef{{ID: lunchID, Name: "Mittagessen"}}, deviation.Offerings)
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
	assert.Equal(t, []timetable.OfferingRef{{ID: randstundeID, Name: "Randstunde"}}, suggestion.Offerings)
	assert.Equal(t, []string{"Klasse 1b"}, suggestion.SchoolClasses)
	assert.Zero(t, suggestion.UnbookedCount)
	assert.Zero(t, suggestion.MissingCount)

	small := classRoster([]int{1, 3}, class[:2], "Klasse 1b")
	assert.Nil(t, timetable.SuggestOfferingSource(small, bookings[:2]), "fewer than three children")
}

// TestSuggestOfferingSourceCombinesForCohortTargets (#5138, #5139): a Klasse
// or Jahrgang Regeltermin plans every child of the cohort, and the bookings
// are narrowed to that cohort. Every offering of it then covers the roster
// fully, so coverage alone cannot tell Randstunde from Ganztag. Together they
// explain the children: the suggestion names both, and connecting keeps
// every child who books either. A Gruppe or hand-picked children keep a
// single offering when the other one is booked mostly elsewhere.
func TestSuggestOfferingSourceCombinesForCohortTargets(t *testing.T) {
	t.Parallel()
	grade := 3
	cohort := studentRange(1001, 20)
	cohortBookings := func() []timetable.OfferingWeekdayBooking {
		bookings := []timetable.OfferingWeekdayBooking{}
		add := func(offeringID int64, name string, students []int64) {
			for _, studentID := range students {
				entry := booking(offeringID, name, studentID, "3a", 1, 2, 3)
				entry.GradeLevel = &grade
				entry.CountsAsCare = true
				bookings = append(bookings, entry)
			}
		}
		// Randstunde: 8 children of the cohort, Ganztag: 14 of them.
		add(randstundeID, "Randstunde", cohort[:8])
		add(ganztagID, "Ganztag bis 14.30 Uhr", cohort[6:])
		return bookings
	}
	both := []timetable.OfferingRef{
		{ID: ganztagID, Name: "Ganztag bis 14.30 Uhr"},
		{ID: randstundeID, Name: "Randstunde"},
	}

	t.Run("Klasse", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{1, 2, 3}, cohort, "3a")
		roster.OfferingKind = timetable.RosterOfferingCare

		suggestion := timetable.SuggestOfferingSource(roster, cohortBookings())

		require.NotNil(t, suggestion)
		assert.False(t, suggestion.Ambiguous, "the combination is a clear suggestion")
		assert.Equal(t, both, suggestion.Offerings)
		assert.Equal(t, both, suggestion.Candidates)
		assert.Equal(t, []string{"3a"}, suggestion.SchoolClasses)
		assert.Zero(t, suggestion.UnbookedCount, "every child books one of the two")
		assert.Nil(t, timetable.DetectRosterDeviation(roster, cohortBookings()),
			"the class list matches both offerings together")
	})

	t.Run("Jahrgang", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{1, 2, 3}, cohort)
		roster.TargetGradeLevels = []int{3}
		roster.OfferingKind = timetable.RosterOfferingCare

		suggestion := timetable.SuggestOfferingSource(roster, cohortBookings())

		require.NotNil(t, suggestion)
		assert.False(t, suggestion.Ambiguous)
		assert.Equal(t, both, suggestion.Offerings)
		assert.Equal(t, []int{3}, suggestion.GradeLevels)
	})

	// A Gruppe has no Klasse or Jahrgang filter: every booking of the school
	// counts, so Ganztag, booked across classes, falls below the rule and
	// would bring too many children to join.
	t.Run("Gruppe", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{1, 2, 3}, cohort)
		roster.OfferingKind = timetable.RosterOfferingCare
		bookings := cohortBookings()
		for _, studentID := range studentRange(5001, 10) {
			entry := booking(ganztagID, "Ganztag bis 14.30 Uhr", studentID, "4b", 1, 2, 3)
			entry.CountsAsCare = true
			bookings = append(bookings, entry)
		}

		suggestion := timetable.SuggestOfferingSource(roster, bookings)

		require.NotNil(t, suggestion)
		assert.False(t, suggestion.Ambiguous)
		assert.Equal(t, []timetable.OfferingRef{{ID: randstundeID, Name: "Randstunde"}}, suggestion.Offerings)
		assert.Equal(t, []timetable.OfferingRef{{ID: randstundeID, Name: "Randstunde"}}, suggestion.Candidates)
		assert.Equal(t, 12, suggestion.UnbookedCount)
	})

	// A third offering passes the rule with less than full coverage and
	// explains no further child: it stays a choice, not part of the
	// suggestion.
	t.Run("Klasse mit schwächerem Angebot", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{1, 2, 3}, cohort, "3a")
		roster.OfferingKind = timetable.RosterOfferingCare
		bookings := cohortBookings()
		for _, studentID := range append(slices.Clone(cohort[:5]), 7001) {
			entry := booking(spaetID, "Spätbetreuung", studentID, "3a", 1, 2, 3)
			entry.GradeLevel = &grade
			entry.CountsAsCare = true
			bookings = append(bookings, entry)
		}

		suggestion := timetable.SuggestOfferingSource(roster, bookings)

		require.NotNil(t, suggestion)
		assert.False(t, suggestion.Ambiguous)
		assert.Equal(t, both, suggestion.Offerings, "Spätbetreuung covers 5 of 6 and adds no child")
		assert.Equal(t, append(slices.Clone(both), timetable.OfferingRef{ID: spaetID, Name: "Spätbetreuung"}), suggestion.Candidates)
		assert.Zero(t, suggestion.MissingCount, "child 7001 is booked only by Spätbetreuung")
	})

	// Hand-picked children: Randstunde covers them fully, Ganztag only 9 of
	// its 14 children; together they would cover 45 of 60 child-weekdays.
	t.Run("Einzelkind", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{1, 2, 3}, cohort[:15])
		roster.OfferingKind = timetable.RosterOfferingCare

		suggestion := timetable.SuggestOfferingSource(roster, cohortBookings())

		require.NotNil(t, suggestion)
		assert.False(t, suggestion.Ambiguous)
		assert.Equal(t, []timetable.OfferingRef{{ID: randstundeID, Name: "Randstunde"}}, suggestion.Offerings)
		assert.Equal(t, 7, suggestion.UnbookedCount)
	})
}

// TestDetectRosterDeviationKlasseRandstundeMitGanztag is the acceptance case
// of #5139: the Randstunde of class 1a runs Monday, Wednesday and Thursday
// 11:45 to 12:45 and is kept by hand with the whole class. Some children
// book the Randstunde (pickup 12:45), some Ganztag from 12:45 on, some both;
// either way they are cared for in that hour. Three children book nothing.
// The hint suggests Randstunde and Ganztag together, counts only the three
// children without a fitting booking, and connecting both drops nobody else.
func TestDetectRosterDeviationKlasseRandstundeMitGanztag(t *testing.T) {
	t.Parallel()
	weekdays := []int{1, 3, 4}
	class := studentRange(1001, 20)
	roster := classRoster(weekdays, class, "1a")
	roster.OfferingKind = timetable.RosterOfferingKindFor(timetable.GroupTypeCare, nil)
	care := func(offeringID int64, name string, studentID int64, days ...int) timetable.OfferingWeekdayBooking {
		entry := booking(offeringID, name, studentID, "1a", days...)
		entry.CountsAsCare = true
		entry.PhaseID = 7
		return entry
	}
	bookings := []timetable.OfferingWeekdayBooking{}
	// Randstunde only: 1001-1006, both: 1007-1010, Ganztag only: 1011-1017.
	for _, studentID := range class[:10] {
		bookings = append(bookings, care(randstundeID, "Randstunde", studentID, 1, 3, 4))
	}
	for _, studentID := range class[6:17] {
		bookings = append(bookings, care(ganztagID, "Ganztag", studentID, 1, 2, 3, 4, 5))
	}
	// 1017 books Ganztag only on Monday and Randstunde on Wednesday and
	// Thursday: every planned day has a fitting booking.
	bookings[len(bookings)-1].Weekdays = []int{1}
	bookings = append(bookings, care(randstundeID, "Randstunde", class[16], 3, 4))
	// The lunch is no care and explains nothing.
	for _, studentID := range class {
		lunch := booking(spaetID, "Mittagessen", studentID, "1a", 1, 2, 3, 4, 5)
		lunch.IncludesLunch = true
		lunch.PhaseID = 7
		bookings = append(bookings, lunch)
	}

	deviation := timetable.DetectRosterDeviation(roster, bookings)

	require.NotNil(t, deviation)
	assert.False(t, deviation.Ambiguous)
	assert.ElementsMatch(t, []timetable.OfferingRef{
		{ID: randstundeID, Name: "Randstunde"},
		{ID: ganztagID, Name: "Ganztag"},
	}, deviation.Offerings)
	assert.Equal(t, 3, deviation.UnbookedCount, "only 1018 to 1020 have no fitting booking")
	assert.Zero(t, deviation.MissingCount)
	assert.Equal(t, []string{"1a"}, deviation.SchoolClasses)

	changes := timetable.PreviewOfferingSource(roster, timetable.OfferingSourceFilter{
		OfferingIDs: []int64{randstundeID, ganztagID}, SchoolClasses: deviation.SchoolClasses,
	}, bookings)
	for _, change := range changes {
		assert.Equal(t, []int64{1018, 1019, 1020}, change.Removed, "weekday %d", change.Weekday)
		assert.Empty(t, change.Added, "weekday %d", change.Weekday)
		assert.Equal(t, 17, change.KeptCount, "weekday %d", change.Weekday)
	}
}

// TestSuggestOfferingSourceCombination covers the limits of a combination:
// two Randstunde children of a Ganztag class join although Randstunde alone
// is below the rule; an offering of another enrollment phase never joins; a
// combination that would bring too many unplanned children stops; two
// offerings booking the same children stay a choice (#5138).
func TestSuggestOfferingSourceCombination(t *testing.T) {
	t.Parallel()
	class := studentRange(1001, 10)

	t.Run("gap filler below the rule joins", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{1, 2}, class, "1a")
		bookings := []timetable.OfferingWeekdayBooking{}
		for _, studentID := range class[:8] {
			bookings = append(bookings, booking(ganztagID, "Ganztag", studentID, "1a", 1, 2))
		}
		for _, studentID := range class[8:] {
			bookings = append(bookings, booking(randstundeID, "Randstunde", studentID, "1a", 1, 2))
		}

		deviation := timetable.SuggestOfferingSource(roster, bookings)

		require.NotNil(t, deviation)
		assert.Equal(t, []timetable.OfferingRef{{ID: ganztagID, Name: "Ganztag"}, {ID: randstundeID, Name: "Randstunde"}}, deviation.Offerings)
		assert.Zero(t, deviation.UnbookedCount)
		assert.Equal(t, []timetable.OfferingRef{{ID: ganztagID, Name: "Ganztag"}, {ID: randstundeID, Name: "Randstunde"}}, deviation.Candidates)
	})

	t.Run("another enrollment phase does not join", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{1}, class, "1a")
		bookings := []timetable.OfferingWeekdayBooking{}
		for _, studentID := range class[:8] {
			entry := booking(ganztagID, "Ganztag", studentID, "1a", 1)
			entry.PhaseID = 1
			bookings = append(bookings, entry)
		}
		for _, studentID := range class[8:] {
			entry := booking(randstundeID, "Randstunde Nachmeldung", studentID, "1a", 1)
			entry.PhaseID = 2
			bookings = append(bookings, entry)
		}

		deviation := timetable.DetectRosterDeviation(roster, bookings)

		require.NotNil(t, deviation)
		assert.Equal(t, []timetable.OfferingRef{{ID: ganztagID, Name: "Ganztag"}}, deviation.Offerings)
		assert.Equal(t, 2, deviation.UnbookedCount)
	})

	// Each offering passes alone with 80 %, but together they bring four
	// unplanned children for ten planned ones: 10/14 is below the rule.
	t.Run("combination keeps the rule", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{1}, class)
		bookings := []timetable.OfferingWeekdayBooking{}
		for _, studentID := range append(slices.Clone(class[:8]), 2001, 2002) {
			bookings = append(bookings, booking(ganztagID, "Ganztag", studentID, "", 1))
		}
		for _, studentID := range append(slices.Clone(class[2:]), 3001, 3002) {
			bookings = append(bookings, booking(randstundeID, "Randstunde", studentID, "", 1))
		}

		deviation := timetable.DetectRosterDeviation(roster, bookings)

		require.NotNil(t, deviation)
		assert.Len(t, deviation.Offerings, 1)
		assert.Equal(t, 2, deviation.UnbookedCount)
		assert.Equal(t, 2, deviation.MissingCount)
		assert.Len(t, deviation.Candidates, 2, "the other offering stays a choice")
	})

	t.Run("two offerings with the same children stay a choice", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{1}, class, "1a")
		bookings := []timetable.OfferingWeekdayBooking{}
		for _, studentID := range class[:8] {
			bookings = append(bookings,
				booking(ganztagID, "Ganztag bis 14.30 Uhr", studentID, "1a", 1),
				booking(spaetID, "Ganztag bis 16 Uhr", studentID, "1a", 1))
		}
		for _, studentID := range class[8:] {
			bookings = append(bookings, booking(randstundeID, "Randstunde", studentID, "1a", 1))
		}

		deviation := timetable.SuggestOfferingSource(roster, bookings)

		require.NotNil(t, deviation)
		assert.True(t, deviation.Ambiguous)
		assert.Equal(t, [][]timetable.OfferingRef{
			{{ID: ganztagID, Name: "Ganztag bis 14.30 Uhr"}, {ID: randstundeID, Name: "Randstunde"}},
			{{ID: spaetID, Name: "Ganztag bis 16 Uhr"}, {ID: randstundeID, Name: "Randstunde"}},
		}, deviation.Combinations)
		for _, combination := range deviation.Combinations {
			ids := make([]int64, 0, len(combination))
			for _, offering := range combination {
				ids = append(ids, offering.ID)
			}
			preview := timetable.PreviewOfferingSource(roster, timetable.OfferingSourceFilter{OfferingIDs: ids, SchoolClasses: []string{"1a"}}, bookings)
			for _, weekday := range preview {
				assert.Empty(t, weekday.Removed, "every equal choice keeps all ten children")
			}
		}
		assert.Equal(t, []timetable.OfferingRef{
			{ID: ganztagID, Name: "Ganztag bis 14.30 Uhr"},
			{ID: randstundeID, Name: "Randstunde"},
		}, deviation.Offerings)
		assert.Equal(t, []timetable.OfferingRef{
			{ID: ganztagID, Name: "Ganztag bis 14.30 Uhr"},
			{ID: randstundeID, Name: "Randstunde"},
			{ID: spaetID, Name: "Ganztag bis 16 Uhr"},
		}, deviation.Candidates)
	})
}

// TestSuggestOfferingSourceEquivalentCombinations covers redundant sources and
// equal choices that only meet the eligibility rule as a complete combination.
func TestSuggestOfferingSourceEquivalentCombinations(t *testing.T) {
	t.Parallel()
	class := studentRange(1001, 10)
	refs := map[int64]timetable.OfferingRef{
		randstundeID: {ID: randstundeID, Name: "Randstunde"},
		ganztagID:    {ID: ganztagID, Name: "Ganztag"},
		spaetID:      {ID: spaetID, Name: "Spätbetreuung"},
	}
	type source struct {
		id       int64
		phase    int64
		students []int64
	}
	for _, tt := range []struct {
		name         string
		sources      []source
		chosen       []int64
		combinations [][]int64
	}{
		{
			name: "remove a starting offering contained in its complement",
			sources: []source{
				{randstundeID, 1, class[:3]},
				{ganztagID, 1, append(slices.Clone(class), 2001)},
			},
			chosen: []int64{ganztagID},
		},
		{
			name: "two child complements remain equal choices",
			sources: []source{
				{ganztagID, 1, class[:8]},
				{randstundeID, 1, class[8:]},
				{spaetID, 1, class[8:]},
			},
			chosen:       []int64{ganztagID, randstundeID},
			combinations: [][]int64{{ganztagID, randstundeID}, {ganztagID, spaetID}},
		},
		{
			name: "standalone equal choices may belong to different phases",
			sources: []source{
				{ganztagID, 1, class[:8]},
				{spaetID, 2, class[:8]},
			},
			chosen:       []int64{ganztagID},
			combinations: [][]int64{{ganztagID}, {spaetID}},
		},
		{
			name: "a complement may not mix phases",
			sources: []source{
				{ganztagID, 1, class[:8]},
				{randstundeID, 1, class[8:]},
				{spaetID, 2, class[8:]},
			},
			chosen: []int64{ganztagID, randstundeID},
		},
		// Both complements add one unplanned child, so both combinations
		// are equally close; Spätbetreuung alone (4 of 6) stays below the
		// rule and only qualifies through the combined coverage.
		{
			name: "a complement uses combined coverage",
			sources: []source{
				{ganztagID, 1, class[:8]},
				{randstundeID, 1, append(slices.Clone(class[8:]), 2002)},
				{spaetID, 1, append(slices.Clone(class[8:]), 2001)},
			},
			chosen:       []int64{ganztagID, randstundeID},
			combinations: [][]int64{{ganztagID, randstundeID}, {ganztagID, spaetID}},
		},
		{
			name: "an alternative below combined coverage is not equal",
			sources: []source{
				{ganztagID, 1, class[:8]},
				{randstundeID, 1, class[8:]},
				{spaetID, 1, append(slices.Clone(class[8:]), 2001, 2002, 2003)},
			},
			chosen: []int64{ganztagID, randstundeID},
		},
		// Ganztag keeps children 1 to 8, Spätbetreuung 3 to 10, each adds
		// two others: equally good, and only the offering id could choose,
		// so the school picks (review of #5363).
		{
			name: "equal scores with different planned children are a tie",
			sources: []source{
				{ganztagID, 1, append(slices.Clone(class[:8]), 2001, 2002)},
				{spaetID, 1, append(slices.Clone(class[2:]), 3001, 3002)},
			},
			chosen:       []int64{ganztagID},
			combinations: [][]int64{{ganztagID}, {spaetID}},
		},
		// A combination that also adds an unplanned child is less close.
		{
			name: "a complement adding an unplanned child is no equal choice",
			sources: []source{
				{ganztagID, 1, class[:8]},
				{randstundeID, 1, class[8:]},
				{spaetID, 1, append(slices.Clone(class[8:]), 2001)},
			},
			chosen: []int64{ganztagID, randstundeID},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			roster := classRoster([]int{1, 3}, class, "1a")
			bookings := []timetable.OfferingWeekdayBooking{}
			for _, source := range tt.sources {
				for _, studentID := range source.students {
					entry := booking(source.id, refs[source.id].Name, studentID, "1a", 1, 3)
					entry.PhaseID = source.phase
					bookings = append(bookings, entry)
				}
			}
			suggestion := timetable.SuggestOfferingSource(roster, bookings)
			require.NotNil(t, suggestion)
			want := make([]timetable.OfferingRef, 0, len(tt.chosen))
			for _, id := range tt.chosen {
				want = append(want, refs[id])
			}
			assert.Equal(t, want, suggestion.Offerings)
			assert.Equal(t, len(tt.combinations) > 0, suggestion.Ambiguous)
			assert.Len(t, suggestion.Combinations, len(tt.combinations))
			for i, ids := range tt.combinations {
				wantCombination := make([]timetable.OfferingRef, 0, len(ids))
				for _, id := range ids {
					wantCombination = append(wantCombination, refs[id])
					assert.Contains(t, suggestion.Candidates, refs[id])
				}
				if i < len(suggestion.Combinations) {
					assert.Equal(t, wantCombination, suggestion.Combinations[i])
				}
			}
		})
	}
}

// TestDetectRosterDeviationMissingData: schools without class data, without
// weekdays on a booking or without any booking keep working; such bookings
// explain nothing and the rule decides as before. The rule reads no pickup
// times, so a booking without one counts like any other.
func TestDetectRosterDeviationMissingData(t *testing.T) {
	t.Parallel()
	class := studentRange(1001, 6)

	t.Run("no bookings", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, timetable.DetectRosterDeviation(classRoster([]int{1}, class, "1a"), nil))
		assert.Nil(t, timetable.SuggestOfferingSource(classRoster([]int{1}, class, "1a"), nil))
	})

	t.Run("bookings without class or weekdays", func(t *testing.T) {
		t.Parallel()
		roster := classRoster([]int{1, 3}, class, "1a")
		bookings := []timetable.OfferingWeekdayBooking{}
		for _, studentID := range class[:4] {
			bookings = append(bookings, booking(randstundeID, "Randstunde", studentID, "1a", 1, 3))
		}
		// No class: outside the Klasse target. No weekdays: plans no day.
		bookings = append(bookings,
			booking(ganztagID, "Ganztag", class[4], "", 1, 3),
			booking(ganztagID, "Ganztag", class[5], "1a"),
		)

		deviation := timetable.DetectRosterDeviation(roster, bookings)

		require.NotNil(t, deviation)
		assert.Equal(t, []timetable.OfferingRef{{ID: randstundeID, Name: "Randstunde"}}, deviation.Offerings)
		assert.Equal(t, 2, deviation.UnbookedCount)
	})

	t.Run("roster without planned children", func(t *testing.T) {
		t.Parallel()
		roster := timetable.TemplateFutureRoster{Weekdays: map[int][]int64{1: {}}}
		bookings := []timetable.OfferingWeekdayBooking{booking(randstundeID, "Randstunde", 1001, "", 1)}
		assert.Nil(t, timetable.SuggestOfferingSource(roster, bookings))
	})
}

// TestSuggestOfferingSourceSkipsHolidayPhases (#5139): an offering of an
// enrollment phase of the kind Ferienbetreuung is never suggested, neither
// as the best offering, nor to complete a combination, nor as an equal
// alternative. The seed books "Ferienbetreuung Herbst" Monday to Friday for
// children of the class, like Ganztag. The dialog still lists it by hand.
// Ganztag with holiday care sits in a school-year phase and stays suggested;
// its IncludesHolidayCare flag does not reach the rule.
func TestSuggestOfferingSourceSkipsHolidayPhases(t *testing.T) {
	t.Parallel()
	const ferienID = int64(304)
	class := studentRange(1001, 10)
	ferien := func(studentID int64, weekdays ...int) timetable.OfferingWeekdayBooking {
		entry := booking(ferienID, "Ferienbetreuung Herbst", studentID, "1b", weekdays...)
		entry.CountsAsCare, entry.HolidayPhase, entry.PhaseID = true, true, 9
		return entry
	}
	care := func(offeringID int64, name string, studentID int64, weekdays ...int) timetable.OfferingWeekdayBooking {
		entry := booking(offeringID, name, studentID, "1b", weekdays...)
		entry.CountsAsCare, entry.PhaseID = true, 7
		return entry
	}
	roster := classRoster([]int{1, 2, 3, 4, 5}, class, "1b")
	roster.OfferingKind = timetable.RosterOfferingCare

	t.Run("not as the best offering", func(t *testing.T) {
		t.Parallel()
		bookings := []timetable.OfferingWeekdayBooking{}
		for _, studentID := range class {
			bookings = append(bookings, ferien(studentID, 1, 2, 3, 4, 5))
		}
		assert.Nil(t, timetable.SuggestOfferingSource(roster, bookings))
		assert.Nil(t, timetable.DetectRosterDeviation(roster, bookings))
	})

	// The holiday care books every child of the class, Ganztag seven: the
	// holiday care would win and Ganztag, of another phase, could not join.
	t.Run("Ganztag stays suggested when the holiday care books more children", func(t *testing.T) {
		t.Parallel()
		bookings := []timetable.OfferingWeekdayBooking{}
		for _, studentID := range class[:7] {
			bookings = append(bookings, care(ganztagID, "Ganztag", studentID, 1, 2, 3, 4, 5))
		}
		for _, studentID := range class {
			bookings = append(bookings, ferien(studentID, 1, 2, 3, 4, 5))
		}

		deviation := timetable.DetectRosterDeviation(roster, bookings)

		require.NotNil(t, deviation)
		assert.Equal(t, []timetable.OfferingRef{{ID: ganztagID, Name: "Ganztag"}}, deviation.Offerings)
		assert.Equal(t, []timetable.OfferingRef{{ID: ganztagID, Name: "Ganztag"}}, deviation.Candidates)
		assert.Equal(t, 3, deviation.UnbookedCount, "the children booking only the holiday care have no fitting booking")
		assert.False(t, deviation.Ambiguous)
	})

	t.Run("not as an equal alternative", func(t *testing.T) {
		t.Parallel()
		bookings := []timetable.OfferingWeekdayBooking{}
		for _, studentID := range class {
			bookings = append(bookings,
				care(ganztagID, "Ganztag", studentID, 1, 2, 3, 4, 5),
				ferien(studentID, 1, 2, 3, 4, 5))
		}

		suggestion := timetable.SuggestOfferingSource(roster, bookings)

		require.NotNil(t, suggestion)
		assert.False(t, suggestion.Ambiguous, "the holiday care is no choice next to Ganztag")
		assert.Equal(t, []timetable.OfferingRef{{ID: ganztagID, Name: "Ganztag"}}, suggestion.Offerings)
		assert.Empty(t, suggestion.Combinations)
	})
}

// TestSuggestOfferingSourceTiedCombinations (review of #5363): equal whole
// combinations are a tie, not only single swaps. Children 1 to 7 are
// planned; A+B and C+D both explain 1 to 6 and add nobody. The rule cannot
// tell them apart, so nothing is preselected and both are offered, also
// when the two combinations lie in different enrollment phases.
func TestSuggestOfferingSourceTiedCombinations(t *testing.T) {
	t.Parallel()
	const (
		offeringA = int64(401)
		offeringB = int64(402)
		offeringC = int64(403)
		offeringD = int64(404)
	)
	planned := studentRange(1, 7)
	roster := classRoster([]int{1}, planned, "1a")
	offer := func(offeringID int64, name string, phaseID int64, students ...int64) []timetable.OfferingWeekdayBooking {
		bookings := make([]timetable.OfferingWeekdayBooking, 0, len(students))
		for _, studentID := range students {
			entry := booking(offeringID, name, studentID, "1a", 1)
			entry.PhaseID = phaseID
			bookings = append(bookings, entry)
		}
		return bookings
	}
	tied := func(phaseAB, phaseCD int64) []timetable.OfferingWeekdayBooking {
		bookings := offer(offeringA, "A", phaseAB, 1, 2, 3)
		bookings = append(bookings, offer(offeringB, "B", phaseAB, 4, 5, 6)...)
		bookings = append(bookings, offer(offeringC, "C", phaseCD, 1, 4, 5)...)
		return append(bookings, offer(offeringD, "D", phaseCD, 2, 3, 6)...)
	}
	ab := []timetable.OfferingRef{{ID: offeringA, Name: "A"}, {ID: offeringB, Name: "B"}}
	cd := []timetable.OfferingRef{{ID: offeringC, Name: "C"}, {ID: offeringD, Name: "D"}}

	for name, phases := range map[string][2]int64{"one phase": {7, 7}, "two phases": {7, 8}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			suggestion := timetable.SuggestOfferingSource(roster, tied(phases[0], phases[1]))

			require.NotNil(t, suggestion)
			assert.True(t, suggestion.Ambiguous, "A+B and C+D explain the same children")
			assert.ElementsMatch(t, [][]timetable.OfferingRef{ab, cd}, suggestion.Combinations)
			assert.Equal(t, 1, suggestion.UnbookedCount, "child 7 books nothing")
		})
	}

	// One more child of C+D breaks the tie: it explains more.
	t.Run("a better combination stays the clear suggestion", func(t *testing.T) {
		t.Parallel()
		bookings := append(tied(7, 7), offer(offeringD, "D", 7, 7)...)

		suggestion := timetable.SuggestOfferingSource(roster, bookings)

		require.NotNil(t, suggestion)
		assert.False(t, suggestion.Ambiguous)
		assert.ElementsMatch(t, cd, suggestion.Offerings)
		assert.Empty(t, suggestion.Combinations)
		assert.Zero(t, suggestion.UnbookedCount)
	})
}

// TestSuggestOfferingSourceSearchBound: beyond twelve offerings in a phase
// the search cannot prove that its choice is unique, so nothing is
// preselected.
func TestSuggestOfferingSourceSearchBound(t *testing.T) {
	t.Parallel()
	const ganztag, firstCourse = int64(500), int64(501)
	planned := studentRange(1, 15)
	roster := classRoster([]int{1}, planned, "1a")
	bookings := []timetable.OfferingWeekdayBooking{}
	// One offering books three children, so it passes the rule; twelve
	// more book one child each: 13 offerings in one phase.
	for _, studentID := range planned[:3] {
		bookings = append(bookings, booking(ganztag, "Ganztag", studentID, "1a", 1))
	}
	for index, studentID := range planned[3:] {
		bookings = append(bookings, booking(firstCourse+int64(index), fmt.Sprintf("Kurs %d", index), studentID, "1a", 1))
	}

	suggestion := timetable.SuggestOfferingSource(roster, bookings)

	require.NotNil(t, suggestion)
	assert.True(t, suggestion.Ambiguous, "13 offerings exceed the search bound")
	assert.Len(t, suggestion.Offerings, timetable.MaxSourceSearchOfferings)
	assert.Equal(t, [][]timetable.OfferingRef{suggestion.Offerings}, suggestion.Combinations,
		"the bounded choice must keep its complementary offerings together")
}

// A higher matched-slot count must not hide the only offering passing #5103.
func TestSuggestOfferingSourceSearchBoundRetainsQualifyingOffering(t *testing.T) {
	t.Parallel()
	const qualifying = int64(600)
	roster := classRoster([]int{1}, studentRange(1, 4), "1a")
	bookings := []timetable.OfferingWeekdayBooking{}
	for _, studentID := range studentRange(1, 3) {
		bookings = append(bookings, booking(qualifying, "Passendes Angebot", studentID, "1a", 1))
	}
	for index := range timetable.MaxSourceSearchOfferings {
		for _, studentID := range studentRange(1, 6) {
			bookings = append(bookings, booking(qualifying+1+int64(index), fmt.Sprintf("Kurs %d", index), studentID, "1a", 1))
		}
	}

	suggestion := timetable.SuggestOfferingSource(roster, bookings)

	require.NotNil(t, suggestion)
	assert.Equal(t, []timetable.OfferingRef{{ID: qualifying, Name: "Passendes Angebot"}}, suggestion.Offerings)
	assert.True(t, suggestion.Ambiguous)
	assert.Equal(t, 1, suggestion.UnbookedCount)
	assert.Zero(t, suggestion.MissingCount)
	assert.Equal(t, [][]timetable.OfferingRef{suggestion.Offerings}, suggestion.Combinations)
}

// TestSuggestOfferingSourceTakesEveryCarryingOffering: six class Randstunden
// (11:45 to 12:45) of one school are the same kind of list. In 2a and 2c
// every child booking "Ganztag bis 16 Uhr" also books the Randstunde today,
// so the fewest offerings that explain the roster leave it out. A later
// child booking only that Ganztag would then drop out of the Randstunde.
// Every offering of the Klasse that carries the Regeltermin by the rule of
// #5342 (fitting kind, pickup time no earlier than the end) joins; one that
// is picked up before the end does not, even if it would cover a child.
func TestSuggestOfferingSourceTakesEveryCarryingOffering(t *testing.T) {
	t.Parallel()
	const (
		ganztag1430ID = int64(304)
		ganztag16ID   = int64(305)
		musicID       = int64(306)
	)
	weekdays := []int{1, 2, 3, 4, 5}
	class := studentRange(1001, 20)
	care := func(offeringID int64, name, pickup string, studentID int64, days ...int) timetable.OfferingWeekdayBooking {
		entry := booking(offeringID, name, studentID, "2a", days...)
		entry.CountsAsCare = true
		entry.PhaseID = 7
		entry.PickupTimes = map[int]string{}
		for _, day := range weekdays {
			entry.PickupTimes[day] = pickup
		}
		return entry
	}
	roster := func() timetable.TemplateFutureRoster {
		roster := classRoster(weekdays, class, "2a")
		roster.OfferingKind = timetable.RosterOfferingKindFor(timetable.GroupTypeCare, nil)
		roster.Ends = map[int]string{1: "12:45", 2: "12:45", 3: "12:45", 4: "12:45", 5: "12:45"}
		return roster
	}
	classBookings := func() []timetable.OfferingWeekdayBooking {
		bookings := []timetable.OfferingWeekdayBooking{}
		// Randstunde: 1001-1012; Ganztag bis 14.30 without Randstunde:
		// 1013-1016; Ganztag bis 16 Uhr: 1005-1008, all with Randstunde.
		for _, studentID := range class[:12] {
			bookings = append(bookings, care(randstundeID, "Randstunde", "12:45", studentID, weekdays...))
		}
		for _, studentID := range class[12:16] {
			bookings = append(bookings, care(ganztag1430ID, "Ganztag bis 14.30", "14:30", studentID, weekdays...))
		}
		for _, studentID := range class[4:8] {
			bookings = append(bookings, care(ganztag16ID, "Ganztag bis 16 Uhr", "16:00", studentID, weekdays...))
		}
		return bookings
	}
	allCarrying := []timetable.OfferingRef{
		{ID: randstundeID, Name: "Randstunde"},
		{ID: ganztag1430ID, Name: "Ganztag bis 14.30"},
		{ID: ganztag16ID, Name: "Ganztag bis 16 Uhr"},
	}

	t.Run("2c: a Ganztag adding nobody today joins", func(t *testing.T) {
		t.Parallel()
		suggestion := timetable.SuggestOfferingSource(roster(), classBookings())

		require.NotNil(t, suggestion)
		assert.False(t, suggestion.Ambiguous)
		assert.ElementsMatch(t, allCarrying, suggestion.Offerings)
		assert.Equal(t, 4, suggestion.UnbookedCount, "1017 to 1020 book nothing")
		assert.Zero(t, suggestion.MissingCount)
	})

	t.Run("2a: an offering picked up before the end stays out", func(t *testing.T) {
		t.Parallel()
		bookings := classBookings()
		// Monday care until the private music lesson at 12:15 books 1017.
		music := care(musicID, "Montags: Betreuung bis zum Beginn des privaten Musikunterrichts", "12:15", class[16], 1)
		bookings = append(bookings, music)

		suggestion := timetable.SuggestOfferingSource(roster(), bookings)

		require.NotNil(t, suggestion)
		assert.False(t, suggestion.Ambiguous)
		assert.ElementsMatch(t, allCarrying, suggestion.Offerings)
		assert.NotContains(t, suggestion.Candidates, timetable.OfferingRef{ID: musicID, Name: music.OfferingName})

		changes := timetable.PreviewOfferingSource(roster(), timetable.OfferingSourceFilter{
			OfferingIDs: []int64{randstundeID, ganztag1430ID, ganztag16ID}, SchoolClasses: suggestion.SchoolClasses,
		}, append(bookings, care(ganztag16ID, "Ganztag bis 16 Uhr", "16:00", 1021, weekdays...)))
		for _, change := range changes {
			assert.Equal(t, []int64{1021}, change.Added, "a later Ganztag-only child stays in the Randstunde, weekday %d", change.Weekday)
			assert.Equal(t, []int64{1017, 1018, 1019, 1020}, change.Removed, "weekday %d", change.Weekday)
		}
	})

	t.Run("without a known end the fewest offerings stay", func(t *testing.T) {
		t.Parallel()
		withoutEnd := roster()
		withoutEnd.Ends = nil

		suggestion := timetable.SuggestOfferingSource(withoutEnd, classBookings())

		require.NotNil(t, suggestion)
		assert.ElementsMatch(t, allCarrying[:2], suggestion.Offerings)
	})

	t.Run("a hand-picked list without Klasse takes no extra offering", func(t *testing.T) {
		t.Parallel()
		handPicked := roster()
		handPicked.TargetSchoolClasses = nil

		suggestion := timetable.SuggestOfferingSource(handPicked, classBookings())

		require.NotNil(t, suggestion)
		assert.ElementsMatch(t, allCarrying[:2], suggestion.Offerings)
	})
}
