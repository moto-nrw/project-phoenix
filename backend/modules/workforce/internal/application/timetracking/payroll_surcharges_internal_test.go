package timetracking

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Surcharge windows (#4807). Every expectation is worked out by hand from the
// stamped times, not recomputed the way the code does.

// October 2026: Saturday 3 is the Tag der Deutschen Einheit, Sunday 4 a plain
// Sunday, Monday 5 a workday.
var (
	surchargeMonthFrom = calendar.NewDate(2026, time.October, 1)
	surchargeMonthTo   = calendar.NewDate(2026, time.October, 31)
	surchargeHolidays  = map[calendar.Date]bool{calendar.NewDate(2026, time.October, 3): true}
)

func nightRule(id int64, rateHundredths int) PayrollSurchargeRule {
	return PayrollSurchargeRule{ID: id, Kind: SurchargeKindNight, WindowStartMinute: 20 * 60, WindowEndMinute: 6 * 60, RateType: SurchargeRatePercent, RateHundredths: rateHundredths, WageTypeID: 1}
}

func dayRule(id int64, kind string, rateType string, rateHundredths int) PayrollSurchargeRule {
	return PayrollSurchargeRule{ID: id, Kind: kind, RateType: rateType, RateHundredths: rateHundredths, WageTypeID: 2}
}

func surchargesFor(sessions []*WorkSession, breaks map[int64][]*WorkSessionBreak, rules []PayrollSurchargeRule) map[int64]int {
	now := laborAt(calendar.NewDate(2026, time.November, 1), 12, 0)
	return SurchargeMinutes(sessions, breaks, surchargeHolidays, rules, surchargeMonthFrom, surchargeMonthTo, now)
}

func TestSurchargeMinutes_NightWindowCountsOnlyTheMinutesInsideIt(t *testing.T) {
	t.Parallel()
	monday := calendar.NewDate(2026, time.October, 5)
	sessions := []*WorkSession{closedBlock(1, laborAt(monday, 18, 0), laborAt(monday, 23, 0))}

	got := surchargesFor(sessions, nil, []PayrollSurchargeRule{nightRule(10, 2500)})

	// 20:00 to 23:00.
	assert.Equal(t, map[int64]int{10: 180}, got)
}

func TestSurchargeMinutes_NightAcrossMidnightLeavesTheBreakOut(t *testing.T) {
	t.Parallel()
	monday := calendar.NewDate(2026, time.October, 5)
	tuesday := monday.AddDays(1)
	sessions := []*WorkSession{closedBlock(1, laborAt(monday, 22, 0), laborAt(tuesday, 7, 0))}
	breaks := map[int64][]*WorkSessionBreak{1: {endedBreak(laborAt(tuesday, 2, 0), 30)}}

	got := surchargesFor(sessions, breaks, []PayrollSurchargeRule{nightRule(10, 2500)})

	// 22:00 to 06:00 is 480 minutes, minus the 30-minute break.
	assert.Equal(t, map[int64]int{10: 450}, got)
}

func TestSurchargeMinutes_SundayAndHolidayCountWholeDays(t *testing.T) {
	t.Parallel()
	holiday := calendar.NewDate(2026, time.October, 3)
	sunday := calendar.NewDate(2026, time.October, 4)
	sessions := []*WorkSession{
		closedBlock(1, laborAt(holiday, 9, 0), laborAt(holiday, 13, 0)),
		closedBlock(2, laborAt(sunday, 10, 0), laborAt(sunday, 12, 30)),
	}
	rules := []PayrollSurchargeRule{
		dayRule(20, SurchargeKindSunday, SurchargeRatePercent, 5000),
		dayRule(30, SurchargeKindHoliday, SurchargeRatePercent, 12500),
	}

	got := surchargesFor(sessions, nil, rules)

	assert.Equal(t, map[int64]int{20: 150, 30: 240}, got)
}

func TestSurchargeMinutes_TheHigherRuleWinsAndNightComesOnTop(t *testing.T) {
	t.Parallel()
	// A holiday on a Sunday: the test declares Sunday the 4th a holiday.
	sunday := calendar.NewDate(2026, time.October, 4)
	holidays := map[calendar.Date]bool{sunday: true}
	sessions := []*WorkSession{closedBlock(1, laborAt(sunday, 18, 0), laborAt(sunday, 22, 0))}
	rules := []PayrollSurchargeRule{
		dayRule(20, SurchargeKindSunday, SurchargeRatePercent, 5000),
		dayRule(30, SurchargeKindHoliday, SurchargeRatePercent, 12500),
		nightRule(10, 2500),
	}
	now := laborAt(calendar.NewDate(2026, time.November, 1), 12, 0)

	got := SurchargeMinutes(sessions, nil, holidays, rules, surchargeMonthFrom, surchargeMonthTo, now)

	// Holiday (125 %) beats Sunday (50 %) for all four hours; the two night
	// hours from 20:00 come on top.
	assert.Equal(t, map[int64]int{30: 240, 10: 120}, got)
}

func TestSurchargeMinutes_ALowerHolidayRuleLosesToTheSundayRule(t *testing.T) {
	t.Parallel()
	sunday := calendar.NewDate(2026, time.October, 4)
	holidays := map[calendar.Date]bool{sunday: true}
	sessions := []*WorkSession{closedBlock(1, laborAt(sunday, 8, 0), laborAt(sunday, 10, 0))}
	rules := []PayrollSurchargeRule{
		dayRule(20, SurchargeKindSunday, SurchargeRateAmount, 300),
		dayRule(30, SurchargeKindHoliday, SurchargeRateAmount, 200),
	}
	now := laborAt(calendar.NewDate(2026, time.November, 1), 12, 0)

	got := SurchargeMinutes(sessions, nil, holidays, rules, surchargeMonthFrom, surchargeMonthTo, now)

	assert.Equal(t, map[int64]int{20: 120}, got)
}

func TestSurchargeMinutes_OnlyTheMonthPartOfASessionCounts(t *testing.T) {
	t.Parallel()
	lastDay := calendar.NewDate(2026, time.October, 31)
	sessions := []*WorkSession{closedBlock(1, laborAt(lastDay, 22, 0), laborAt(lastDay.AddDays(1), 2, 0))}

	got := surchargesFor(sessions, nil, []PayrollSurchargeRule{nightRule(10, 2500)})

	// 22:00 to midnight of 31 October; 1 November belongs to the next month.
	assert.Equal(t, map[int64]int{10: 120}, got)
}

func TestSurchargeMinutes_ACustomNightWindowWithinOneDay(t *testing.T) {
	t.Parallel()
	monday := calendar.NewDate(2026, time.October, 5)
	sessions := []*WorkSession{closedBlock(1, laborAt(monday, 0, 30), laborAt(monday, 8, 0))}
	rule := nightRule(10, 4000)
	rule.WindowStartMinute, rule.WindowEndMinute = 0, 4*60

	got := surchargesFor(sessions, nil, []PayrollSurchargeRule{rule})

	assert.Equal(t, map[int64]int{10: 210}, got)
}

func TestSurchargeMinutes_DaylightSavingNightHasItsRealLength(t *testing.T) {
	t.Parallel()
	// The night from Saturday 24 to Sunday 25 October 2026 has 25 hours.
	saturday := calendar.NewDate(2026, time.October, 24)
	start := saturday.BerlinMidnight().Add(22 * time.Hour)
	end := start.Add(10 * time.Hour) // 22:00 CEST to 07:00 CET
	sessions := []*WorkSession{closedBlock(1, start, end)}

	got := surchargesFor(sessions, nil, []PayrollSurchargeRule{nightRule(10, 2500)})

	// 22:00 to 06:00 wall clock is nine real hours that night.
	assert.Equal(t, map[int64]int{10: 540}, got)
}

func TestSurchargeMinutes_NoRulesNoMinutes(t *testing.T) {
	t.Parallel()
	monday := calendar.NewDate(2026, time.October, 5)
	sessions := []*WorkSession{closedBlock(1, laborAt(monday, 22, 0), laborAt(monday, 23, 0))}

	assert.Empty(t, surchargesFor(sessions, nil, nil))
}

func TestSurchargeAmountCents_RoundsToTheCent(t *testing.T) {
	t.Parallel()
	// 2,50 € per hour for 1 h 10 min is 2,9166… € → 2,92 €.
	assert.Equal(t, 292, SurchargeAmountCents(70, 250))
	assert.Equal(t, 0, SurchargeAmountCents(0, 250))
}
