package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Which calendar days one dashboard view speaks for (#1542). The weekend rule
// is the whole point of the feature: nobody opens the app on Saturday, so a
// Monday has to carry the two days before it or those birthdays are never seen.
func TestCelebrationDates(t *testing.T) {
	t.Parallel()

	t.Run("an ordinary day speaks only for itself", func(t *testing.T) {
		wednesday := calendar.NewDate(2026, time.August, 5)
		assert.Equal(t, []calendar.Date{wednesday}, CelebrationDates(wednesday))
	})

	t.Run("monday carries the weekend before it", func(t *testing.T) {
		monday := calendar.NewDate(2026, time.August, 3)
		got := CelebrationDates(monday)

		assert.Equal(t, []calendar.Date{
			monday,
			calendar.NewDate(2026, time.August, 1), // Saturday
			calendar.NewDate(2026, time.August, 2), // Sunday
		}, got)
	})

	t.Run("sunday does not pre-empt the monday view", func(t *testing.T) {
		sunday := calendar.NewDate(2026, time.August, 2)
		assert.Len(t, CelebrationDates(sunday), 1)
	})
}

// A leap-day birth date must not disappear for three years out of four.
func TestMonthDaysFor(t *testing.T) {
	t.Parallel()

	t.Run("1 March stands in for 29 February in a common year", func(t *testing.T) {
		got := MonthDaysFor(calendar.NewDate(2027, time.March, 1))

		assert.Equal(t, []MonthDay{
			{Month: time.March, Day: 1},
			{Month: time.February, Day: 29},
		}, got)
	})

	t.Run("in a leap year 1 March stands only for itself", func(t *testing.T) {
		got := MonthDaysFor(calendar.NewDate(2028, time.March, 1))

		assert.Equal(t, []MonthDay{{Month: time.March, Day: 1}}, got)
		assert.NotContains(t, got, MonthDay{Month: time.February, Day: 29})
	})

	t.Run("the leap day itself is used in a leap year", func(t *testing.T) {
		got := MonthDaysFor(calendar.NewDate(2028, time.February, 29))

		assert.Equal(t, []MonthDay{{Month: time.February, Day: 29}}, got)
	})

	t.Run("an ordinary day maps to one recurring day", func(t *testing.T) {
		got := MonthDaysFor(calendar.NewDate(2026, time.August, 5))

		assert.Equal(t, []MonthDay{{Month: time.August, Day: 5}}, got)
	})
}

func TestIsLeapYear(t *testing.T) {
	t.Parallel()

	assert.True(t, isLeapYear(2028))
	assert.True(t, isLeapYear(2000), "a year divisible by 400 is a leap year")
	assert.False(t, isLeapYear(1900), "a century year not divisible by 400 is not")
	assert.False(t, isLeapYear(2027))
}

// Ordering and disclosure rules: today first, children before staff, and no
// age on a staff entry.
func TestBuildCelebrations(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.August, 3)
	saturday := calendar.NewDate(2026, time.August, 1)
	byMonthDay := map[MonthDay]calendar.Date{
		{Month: time.August, Day: 3}: monday,
		{Month: time.August, Day: 1}: saturday,
	}

	entries := []BirthdayEntry{
		{
			Kind:      BirthdayKindStaff,
			ID:        7,
			FirstName: "Anna",
			LastName:  "Berg",
			Birthday:  calendar.NewDate(1988, time.August, 3),
		},
		{
			Kind:        BirthdayKindStudent,
			ID:          2,
			FirstName:   "Mika",
			LastName:    "Klein",
			Birthday:    calendar.NewDate(2019, time.August, 1),
			GroupName:   "Delfine",
			SchoolClass: "1a",
		},
		{
			Kind:      BirthdayKindStudent,
			ID:        1,
			FirstName: "Lina",
			LastName:  "Adler",
			Birthday:  calendar.NewDate(2018, time.August, 3),
		},
	}

	got := BuildCelebrations(entries, byMonthDay, monday)

	if assert.Len(t, got, 3) {
		assert.Equal(t, "Lina Adler", got[0].Name, "today's children come first")
		assert.True(t, got[0].IsToday)
		assert.Equal(t, 8, got[0].Age, "a child turning eight on the shown day")

		assert.Equal(t, "Anna Berg", got[1].Name, "staff follow the children of the same day")
		assert.Equal(t, 0, got[1].Age, "a staff age is never disclosed")
		assert.True(t, got[1].IsToday)

		assert.Equal(t, "Mika Klein", got[2].Name, "the weekend entry comes last")
		assert.False(t, got[2].IsToday)
		assert.Equal(t, saturday, got[2].Date, "shown on the day it actually happened")
		assert.Equal(t, 7, got[2].Age)
	}
}

// A leap-day child shown on 1 March turns the age of that March, not of the
// February that did not happen.
func TestBuildCelebrationsLeapDayAge(t *testing.T) {
	t.Parallel()

	firstOfMarch := calendar.NewDate(2027, time.March, 1)
	byMonthDay := map[MonthDay]calendar.Date{
		{Month: time.March, Day: 1}:     firstOfMarch,
		{Month: time.February, Day: 29}: firstOfMarch,
	}

	got := BuildCelebrations([]BirthdayEntry{{
		Kind:      BirthdayKindStudent,
		ID:        3,
		FirstName: "Jonas",
		LastName:  "Feld",
		Birthday:  calendar.NewDate(2020, time.February, 29),
	}}, byMonthDay, firstOfMarch)

	if assert.Len(t, got, 1) {
		assert.Equal(t, firstOfMarch, got[0].Date)
		assert.True(t, got[0].IsToday)
		assert.Equal(t, 7, got[0].Age)
	}
}

// An entry the day mapping does not know is dropped rather than rendered on an
// invented date — that would silently paper over a filter/mapping mismatch.
func TestBuildCelebrationsDropsUnmappedDays(t *testing.T) {
	t.Parallel()

	today := calendar.NewDate(2026, time.August, 5)
	byMonthDay := map[MonthDay]calendar.Date{
		{Month: time.August, Day: 5}: today,
	}

	got := BuildCelebrations([]BirthdayEntry{{
		Kind:      BirthdayKindStudent,
		ID:        9,
		FirstName: "Nicht",
		LastName:  "Gefragt",
		Birthday:  calendar.NewDate(2017, time.September, 9),
	}}, byMonthDay, today)

	assert.Empty(t, got)
}
