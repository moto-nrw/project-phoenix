package domain

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

// The week of Monday 2026-09-14; 2026-09-17 is its Thursday.
var (
	monday   = calendar.NewDate(2026, time.September, 14)
	thursday = calendar.NewDate(2026, time.September, 17)
)

func berlin(day Date, hour, minute int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, calendar.Berlin)
}

func TestDeadlineAnchorsDaysBeforeAndPreviousWeek(t *testing.T) {
	t.Parallel()
	sameDay := Deadline{Kind: DeadlineDaysBefore, Clock: "09:00"}
	assert.Equal(t, berlin(thursday, 9, 0), sameDay.At(thursday))

	twoDays := Deadline{Kind: DeadlineDaysBefore, LeadDays: 2, Clock: "12:30"}
	assert.Equal(t, berlin(calendar.NewDate(2026, time.September, 15), 12, 30), twoDays.At(thursday))
	// Weekends count: Monday minus two days is Saturday.
	assert.Equal(t, berlin(calendar.NewDate(2026, time.September, 12), 12, 30), twoDays.At(monday))

	previousThursday := Deadline{Kind: DeadlinePreviousWeek, Weekday: time.Thursday, Clock: "12:00"}
	want := berlin(calendar.NewDate(2026, time.September, 10), 12, 0)
	for day := monday; day.Before(monday.AddDays(5)); day = day.AddDays(1) {
		assert.Equal(t, want, previousThursday.At(day), "every day of the week shares the previous Thursday")
	}
}

func TestDeadlineValidity(t *testing.T) {
	t.Parallel()
	assert.True(t, Deadline{Kind: DeadlineDaysBefore, LeadDays: 14, Clock: "09:00"}.Valid())
	assert.False(t, Deadline{Kind: DeadlineDaysBefore, LeadDays: 15, Clock: "09:00"}.Valid())
	assert.False(t, Deadline{Kind: DeadlineDaysBefore, LeadDays: -1, Clock: "09:00"}.Valid())
	assert.False(t, Deadline{Kind: DeadlineDaysBefore, Clock: "9 Uhr"}.Valid())
	assert.True(t, Deadline{Kind: DeadlinePreviousWeek, Weekday: time.Friday, Clock: "09:00"}.Valid())
	assert.False(t, Deadline{Kind: DeadlinePreviousWeek, Weekday: time.Saturday, Clock: "09:00"}.Valid())
	assert.False(t, Deadline{Kind: "monthly", Clock: "09:00"}.Valid())
}

func TestOrderingRulesValidityChecksOnlyWhatTheModelUses(t *testing.T) {
	t.Parallel()
	rules := defaultOrderingRules("09:00")
	assert.True(t, rules.Valid())

	rules.Cancellation = Deadline{}
	assert.True(t, rules.Valid(), "the single model never reads the cancellation deadline")
	rules.Model = ModelSplit
	assert.False(t, rules.Valid())

	annual := defaultOrderingRules("09:00")
	annual.Model = ModelAnnual
	annual.Registration = Deadline{}
	assert.True(t, annual.Valid(), "the annual model never reads the registration deadline")
	annual.AnnualChanges = "sometimes"
	assert.False(t, annual.Valid())

	broken := defaultOrderingRules("09:00")
	broken.Rhythm = "monthly"
	assert.False(t, broken.Valid())
	broken = defaultOrderingRules("09:00")
	broken.Model = "weekly"
	assert.False(t, broken.Valid())
}

func TestSingleModelKeepsTheSameDayCutoff(t *testing.T) {
	t.Parallel()
	rules := defaultOrderingRules("09:00")
	before, after := berlin(thursday, 8, 59), berlin(thursday, 9, 1)

	assert.True(t, rules.ParentCanRegister(before, thursday))
	assert.True(t, rules.ParentCanCancel(before, thursday))
	assert.False(t, rules.ParentCanRegister(after, thursday))
	assert.False(t, rules.ParentCanCancel(after, thursday))
	assert.False(t, rules.ParentCanRegister(berlin(thursday, 8, 0), thursday.AddDays(-1)), "a past day is never open")
	assert.Equal(t, berlin(thursday, 9, 0), rules.FreezeAt(thursday))
	assert.Equal(t, rules.CancellationAt(thursday), rules.AbsenceCutoff(thursday))
}

func TestSplitModelRegistersEarlyAndCancelsLate(t *testing.T) {
	t.Parallel()
	rules := defaultOrderingRules("12:00")
	rules.Model = ModelSplit
	rules.Registration = Deadline{Kind: DeadlinePreviousWeek, Weekday: time.Thursday, Clock: "12:00"}
	rules.Cancellation = Deadline{Kind: DeadlineDaysBefore, Clock: "08:00"}
	weekBefore := berlin(calendar.NewDate(2026, time.September, 11), 10, 0)

	assert.False(t, rules.ParentCanRegister(weekBefore, thursday), "registration closed on the previous Thursday")
	assert.True(t, rules.ParentCanCancel(weekBefore, thursday))
	assert.True(t, rules.ParentCanCancel(berlin(thursday, 7, 59), thursday))
	assert.False(t, rules.ParentCanCancel(berlin(thursday, 8, 1), thursday))
	assert.Equal(t, berlin(calendar.NewDate(2026, time.September, 10), 12, 0), rules.FreezeAt(thursday), "the order freezes at the earlier deadline")
}

func TestSplitModelFreezesAtACancellationDeadlineThatComesFirst(t *testing.T) {
	t.Parallel()
	rules := defaultOrderingRules("09:00")
	rules.Model = ModelSplit
	rules.Cancellation = Deadline{Kind: DeadlineDaysBefore, LeadDays: 3, Clock: "09:00"}

	assert.Equal(t, rules.Cancellation.At(thursday), rules.FreezeAt(thursday))
	now := berlin(calendar.NewDate(2026, time.September, 15), 10, 0)
	assert.True(t, rules.ParentCanRegister(now, thursday))
	assert.False(t, rules.ParentCanCancel(now, thursday))
}

func TestAnnualModelClosesRegistrationAfterTheDeadline(t *testing.T) {
	t.Parallel()
	rules := defaultOrderingRules("09:00")
	rules.Model = ModelAnnual
	rules.Cancellation = Deadline{Kind: DeadlineDaysBefore, LeadDays: 1, Clock: "18:00"}
	rules.AnnualDeadline = calendar.NewDate(2026, time.August, 31)
	inAugust := berlin(calendar.NewDate(2026, time.August, 31), 23, 0)
	inSeptember := berlin(calendar.NewDate(2026, time.September, 1), 8, 0)

	assert.True(t, rules.ParentCanRegister(inAugust, thursday))
	assert.True(t, rules.ParentCanChangeSchedule(inAugust))
	assert.False(t, rules.ParentCanRegister(inSeptember, thursday))
	assert.False(t, rules.ParentCanChangeSchedule(inSeptember))
	assert.True(t, rules.ParentCanCancel(inSeptember, thursday), "parents still cancel until the cancellation deadline")
	assert.False(t, rules.ParentCanCancel(berlin(calendar.NewDate(2026, time.September, 16), 18, 1), thursday))

	rules.AnnualChanges = AnnualSchoolOnly
	assert.True(t, rules.ParentCanCancel(inAugust, thursday))
	assert.False(t, rules.ParentCanCancel(inSeptember, thursday), "only the school changes after the deadline")

	rules.AnnualDeadline = ""
	assert.True(t, rules.ParentCanRegister(inSeptember, thursday), "without a date the annual registration stays open")
}

func TestAbsenceCutoffFollowsTheSickMode(t *testing.T) {
	t.Parallel()
	rules := defaultOrderingRules("09:00")
	assert.Equal(t, berlin(thursday, 9, 0), rules.AbsenceCutoff(thursday))
	rules.Sick = SickCancelled
	assert.Equal(t, thursday.EndOfDay(), rules.AbsenceCutoff(thursday))
}

func TestNextEffectiveDateSkipsFrozenDaysAndWeekends(t *testing.T) {
	t.Parallel()
	rules := defaultOrderingRules("09:00")
	assert.Equal(t, thursday, rules.NextEffectiveDate(berlin(thursday, 8, 0)))
	assert.Equal(t, thursday.AddDays(1), rules.NextEffectiveDate(berlin(thursday, 10, 0)))
	friday := thursday.AddDays(1)
	assert.Equal(t, monday.AddDays(7), rules.NextEffectiveDate(berlin(friday, 10, 0)), "Friday after the cutoff moves to Monday")

	rules.Registration = Deadline{Kind: DeadlinePreviousWeek, Weekday: time.Thursday, Clock: "12:00"}
	assert.Equal(t, monday.AddDays(7), rules.NextEffectiveDate(berlin(thursday, 11, 0)), "next week is still open before Thursday noon")
	assert.Equal(t, monday.AddDays(14), rules.NextEffectiveDate(berlin(thursday, 13, 0)))
}

// The clock switches back on Sunday 2026-10-25: a deadline on either side
// keeps its Berlin wall-clock time.
func TestDeadlinesKeepTheirClockAcrossTheDaylightSavingSwitch(t *testing.T) {
	t.Parallel()
	monday := calendar.NewDate(2026, time.October, 26)
	previousThursday := Deadline{Kind: DeadlinePreviousWeek, Weekday: time.Thursday, Clock: "12:00"}
	assert.Equal(t, time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC), previousThursday.At(monday).UTC(), "summer time: 12:00 is 10:00 UTC")

	twoDays := Deadline{Kind: DeadlineDaysBefore, LeadDays: 2, Clock: "09:00"}
	assert.Equal(t, time.Date(2026, 10, 24, 7, 0, 0, 0, time.UTC), twoDays.At(monday).UTC(), "Saturday before the switch")
	assert.Equal(t, time.Date(2026, 10, 25, 8, 0, 0, 0, time.UTC), twoDays.At(monday.AddDays(1)).UTC(), "Sunday after the switch: 09:00 is 08:00 UTC")
}

// defaultOrderingRules are the rules of a school that changed nothing: one
// deadline on the meal day at the given clock time, as before #3873.
func defaultOrderingRules(clock string) OrderingRules {
	deadline := Deadline{Kind: DeadlineDaysBefore, Clock: clock}
	return OrderingRules{
		Model: ModelSingle, Registration: deadline, Cancellation: deadline,
		AnnualChanges: AnnualParentsCancel, Sick: SickStaysOrdered, Rhythm: RhythmDaily, Detail: DetailByClass,
	}
}
