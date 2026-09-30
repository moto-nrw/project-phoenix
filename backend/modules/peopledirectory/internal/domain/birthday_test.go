package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Which calendar days one week view speaks for (#3777): Monday to Sunday.
// The weekend belongs to the week around it — nobody opens the app on
// Saturday, and the OGS celebrates during the week.
func TestCelebrationDates(t *testing.T) {
	t.Parallel()

	week, err := ResolveBirthdayWeek(calendar.NewDate(2026, time.August, 5), nil)
	require.NoError(t, err)

	got := CelebrationDates(week)

	require.Len(t, got, 7)
	assert.Equal(t, calendar.NewDate(2026, time.August, 3), got[0], "the week starts on Monday")
	assert.Equal(t, calendar.NewDate(2026, time.August, 9), got[6], "the week ends on Sunday")
}

func TestWeekStartOf(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.August, 3)
	assert.Equal(t, monday, WeekStartOf(monday), "a Monday is its own week start")
	assert.Equal(t, monday, WeekStartOf(calendar.NewDate(2026, time.August, 5)))
	assert.Equal(t, monday, WeekStartOf(calendar.NewDate(2026, time.August, 9)),
		"Sunday closes the week, it does not open the next one")
	assert.Equal(t, calendar.NewDate(2026, time.December, 28), WeekStartOf(calendar.NewDate(2027, time.January, 1)),
		"a week crosses the turn of the year")
}

// The view may step BirthdayWeekReach weeks back and ahead, and no further:
// beyond that the card would publish the staff birthdays of the whole year.
func TestResolveBirthdayWeek(t *testing.T) {
	t.Parallel()

	today := calendar.NewDate(2026, time.September, 29) // Tuesday
	current := calendar.NewDate(2026, time.September, 28)

	t.Run("nil means the current week", func(t *testing.T) {
		week, err := ResolveBirthdayWeek(today, nil)
		require.NoError(t, err)
		assert.Equal(t, BirthdayWeek{Start: current, End: calendar.NewDate(2026, time.October, 4)}, week)
	})

	t.Run("any day of a week selects that week", func(t *testing.T) {
		thursday := calendar.NewDate(2026, time.September, 24)
		week, err := ResolveBirthdayWeek(today, &thursday)
		require.NoError(t, err)
		assert.Equal(t, calendar.NewDate(2026, time.September, 21), week.Start)
	})

	t.Run("the outermost weeks are allowed", func(t *testing.T) {
		earliest := current.AddDays(-7 * BirthdayWeekReach)
		latest := current.AddDays(7*BirthdayWeekReach + 6)
		_, err := ResolveBirthdayWeek(today, &earliest)
		require.NoError(t, err)
		_, err = ResolveBirthdayWeek(today, &latest)
		require.NoError(t, err)
	})

	t.Run("one week further is rejected", func(t *testing.T) {
		tooEarly := current.AddDays(-7*BirthdayWeekReach - 1)
		tooLate := current.AddDays(7 * (BirthdayWeekReach + 1))
		_, err := ResolveBirthdayWeek(today, &tooEarly)
		require.ErrorIs(t, err, ErrBirthdayWeekOutOfRange)
		_, err = ResolveBirthdayWeek(today, &tooLate)
		require.ErrorIs(t, err, ErrBirthdayWeekOutOfRange)
	})

	t.Run("bounds are the outermost Mondays", func(t *testing.T) {
		earliest, latest := BirthdayWeekBounds(today)
		assert.Equal(t, calendar.NewDate(2026, time.August, 31), earliest)
		assert.Equal(t, calendar.NewDate(2026, time.October, 26), latest)
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

// Ordering and disclosure rules: calendar order from Monday to Sunday,
// children before staff on the same day, and no age on a staff entry.
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
		assert.Equal(t, "Mika Klein", got[0].Name, "the earlier day comes first")
		assert.False(t, got[0].IsToday)
		assert.Equal(t, saturday, got[0].Date, "shown on the day it actually happened")
		assert.Equal(t, 7, got[0].Age)

		assert.Equal(t, "Lina Adler", got[1].Name, "children lead their day")
		assert.True(t, got[1].IsToday)
		assert.Equal(t, 8, got[1].Age, "a child turning eight on the shown day")

		assert.Equal(t, "Anna Berg", got[2].Name, "staff follow the children of the same day")
		assert.Equal(t, 0, got[2].Age, "a staff age is never disclosed")
		assert.True(t, got[2].IsToday)
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
