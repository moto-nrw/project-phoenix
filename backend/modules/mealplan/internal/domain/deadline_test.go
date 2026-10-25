package domain

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestDeadlineAtCountsCalendarDaysBack(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		leadDays int
		date     Date
		want     time.Time
	}{
		{name: "no lead keeps the meal day", leadDays: 0, date: "2026-10-21", want: berlin("2026-10-21", 9, 0)},
		{name: "one day before a Wednesday is Tuesday", leadDays: 1, date: "2026-10-21", want: berlin("2026-10-20", 9, 0)},
		{name: "one day before a Monday is Sunday", leadDays: 1, date: "2026-10-26", want: berlin("2026-10-25", 9, 0)},
		{name: "three days before a Tuesday include the weekend", leadDays: 3, date: "2026-10-27", want: berlin("2026-10-24", 9, 0)},
		{name: "five calendar days", leadDays: 5, date: "2026-10-30", want: berlin("2026-10-25", 9, 0)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			deadline := Deadline{Kind: DeadlineDaysBefore, Clock: "09:00", LeadDays: test.leadDays}
			assert.True(t, test.want.Equal(deadline.At(test.date)), "got %s", deadline.At(test.date))
		})
	}
}

func TestDeadlineAtKeepsBerlinWallClockAcrossDST(t *testing.T) {
	t.Parallel()
	// A three-day lead reaches Friday before the daylight saving switch.
	deadline := Deadline{Kind: DeadlineDaysBefore, Clock: "14:30", LeadDays: 3}
	got := deadline.At("2026-10-26")
	assert.Equal(t, "2026-10-23 14:30 CEST", got.In(calendar.Berlin).Format("2006-01-02 15:04 MST"))
}

func TestDeadlineChangeable(t *testing.T) {
	t.Parallel()
	wednesdayMorning := berlin("2026-10-21", 8, 0)
	wednesdayNoon := berlin("2026-10-21", 12, 0)

	assert.True(t, defaultOrderingRules("09:00").ParentCanRegister(wednesdayMorning, "2026-10-21"))
	assert.False(t, defaultOrderingRules("09:00").ParentCanRegister(wednesdayNoon, "2026-10-21"))
	assert.False(t, defaultOrderingRules("09:00").ParentCanRegister(wednesdayMorning, "2026-10-20"), "past days stay fixed")

	assert.False(t, leadRules(1).ParentCanRegister(wednesdayMorning, "2026-10-21"), "the meal day itself is fixed since Tuesday 09:00")
	assert.True(t, leadRules(1).ParentCanRegister(wednesdayMorning, "2026-10-22"))
	assert.False(t, leadRules(1).ParentCanRegister(wednesdayNoon, "2026-10-22"))
	assert.True(t, leadRules(1).ParentCanRegister(wednesdayNoon, "2026-10-23"))
	assert.True(t, leadRules(1).ParentCanRegister(berlin("2026-10-25", 8, 59), "2026-10-26"), "Monday closes Sunday 09:00")
	assert.False(t, leadRules(1).ParentCanRegister(berlin("2026-10-25", 10, 0), "2026-10-26"), "Monday is fixed after Sunday cutoff")
}

func TestDeadlineNextEffectiveDate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		leadDays int
		now      time.Time
		want     Date
	}{
		{name: "same day before cutoff", leadDays: 0, now: berlin("2026-10-21", 8, 0), want: "2026-10-21"},
		{name: "same day after cutoff", leadDays: 0, now: berlin("2026-10-21", 10, 0), want: "2026-10-22"},
		{name: "same day after Friday cutoff", leadDays: 0, now: berlin("2026-10-23", 10, 0), want: "2026-10-26"},
		{name: "weekend", leadDays: 0, now: berlin("2026-10-24", 10, 0), want: "2026-10-26"},
		{name: "one day lead before cutoff", leadDays: 1, now: berlin("2026-10-21", 8, 0), want: "2026-10-22"},
		{name: "one day lead after cutoff", leadDays: 1, now: berlin("2026-10-21", 10, 0), want: "2026-10-23"},
		{name: "one day lead after Friday cutoff", leadDays: 1, now: berlin("2026-10-23", 10, 0), want: "2026-10-26"},
		{name: "one day lead on the weekend", leadDays: 1, now: berlin("2026-10-24", 10, 0), want: "2026-10-26"},
		{name: "five days lead", leadDays: 5, now: berlin("2026-10-21", 10, 0), want: "2026-10-27"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rules := leadRules(test.leadDays)
			assert.Equal(t, test.want, rules.NextEffectiveDate(test.now))
		})
	}
}

func leadRules(days int) OrderingRules {
	rules := defaultOrderingRules("09:00")
	rules.Registration.LeadDays = days
	return rules
}
