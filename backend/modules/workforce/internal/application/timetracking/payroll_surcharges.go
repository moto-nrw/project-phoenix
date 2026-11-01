package timetracking

import (
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Surcharge windows (#4807): how many stamped net minutes of a month fall
// into each surcharge rule. Times are read on the Berlin wall clock; a
// session across midnight is split onto its days the way labor_time_policy
// does it, and breaks are left out.
//
// Precedence (decided in #4807): of the day rules (Sunday, holiday) the
// higher one applies to a minute; a night rule comes on top of it. Several
// night rules for one minute also yield to the higher one. "Higher" compares
// the rate of two rules of the same rate type; between a percent and an
// amount rule there is no common scale, so a holiday rule beats a Sunday rule
// and, among night rules, the percent rule wins.

// The surcharge vocabulary of the public contract.
type PayrollSurchargeRule = workforce.PayrollSurchargeRule

const (
	SurchargeKindNight   = workforce.SurchargeKindNight
	SurchargeKindSunday  = workforce.SurchargeKindSunday
	SurchargeKindHoliday = workforce.SurchargeKindHoliday
	SurchargeRatePercent = workforce.SurchargeRatePercent
	SurchargeRateAmount  = workforce.SurchargeRateAmount
)

const minutesPerDay = 24 * 60

// SurchargeMinutes returns, per rule ID, the net minutes of the sessions in
// [from, to] that the rule covers. Rules without a minute are left out.
// Running sessions count up to now, like the month card counts them.
func SurchargeMinutes(
	sessions []*WorkSession,
	breaksBySession map[int64][]*WorkSessionBreak,
	holidays map[calendar.Date]bool,
	rules []PayrollSurchargeRule,
	from, to calendar.Date,
	now time.Time,
) map[int64]int {
	result := make(map[int64]int)
	if len(rules) == 0 {
		return result
	}
	classifier := newSurchargeClassifier(rules, holidays)
	lower, upper := from.BerlinMidnight(), to.AddDays(1).BerlinMidnight()
	seconds := make(map[int64]int64)
	for _, session := range sessions {
		for _, span := range netWorkSpans(session, breaksBySession[session.ID], now) {
			span = span.clip(lower, upper)
			if span.empty() {
				continue
			}
			classifier.add(span, seconds)
		}
	}
	for ruleID, total := range seconds {
		if minutes := int((total + 30) / 60); minutes > 0 {
			result[ruleID] = minutes
		}
	}
	return result
}

// SurchargeAmountCents is the amount an amount rule books for the minutes:
// cents per hour times hours, rounded half up to the cent.
func SurchargeAmountCents(minutes, centsPerHour int) int {
	if minutes <= 0 || centsPerHour <= 0 {
		return 0
	}
	return (minutes*centsPerHour + 30) / 60
}

type timeSpan struct{ start, end time.Time }

func (s timeSpan) empty() bool { return !s.end.After(s.start) }

func (s timeSpan) clip(lower, upper time.Time) timeSpan {
	if s.start.Before(lower) {
		s.start = lower
	}
	if s.end.After(upper) {
		s.end = upper
	}
	return s
}

// netWorkSpans is the session from check-in to its balance end with the
// break intervals cut out. A legacy row that only carries a cached break
// total loses that many minutes from its start, as the day mapping of
// netMinutesByDate takes them from its first day.
func netWorkSpans(session *WorkSession, breaks []*WorkSessionBreak, now time.Time) []timeSpan {
	end := BalanceSessionEnd(session, now)
	start := session.CheckInTime
	if len(breaks) == 0 && session.BreakMinutes > 0 {
		start = start.Add(time.Duration(session.BreakMinutes) * time.Minute)
	}
	if !end.After(start) {
		return nil
	}
	cuts := breakSpans(breaks, end)
	spans := []timeSpan{}
	cursor := start
	for _, cut := range cuts {
		if cut.start.After(cursor) {
			spans = append(spans, timeSpan{start: cursor, end: minTime(cut.start, end)})
		}
		if cut.end.After(cursor) {
			cursor = cut.end
		}
		if !cursor.Before(end) {
			break
		}
	}
	if end.After(cursor) {
		spans = append(spans, timeSpan{start: cursor, end: end})
	}
	return spans
}

// breakSpans are the break intervals in start order; a running break lasts
// until the session end.
func breakSpans(breaks []*WorkSessionBreak, end time.Time) []timeSpan {
	cuts := make([]timeSpan, 0, len(breaks))
	for _, brk := range breaks {
		breakEnd := end
		if brk.EndedAt != nil && brk.EndedAt.Before(breakEnd) {
			breakEnd = *brk.EndedAt
		}
		if cut := (timeSpan{start: brk.StartedAt, end: breakEnd}); !cut.empty() {
			cuts = append(cuts, cut)
		}
	}
	slices.SortFunc(cuts, func(a, b timeSpan) int { return a.start.Compare(b.start) })
	return cuts
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

type surchargeClassifier struct {
	night, sunday, holiday []PayrollSurchargeRule
	holidays               map[calendar.Date]bool
}

func newSurchargeClassifier(rules []PayrollSurchargeRule, holidays map[calendar.Date]bool) *surchargeClassifier {
	c := &surchargeClassifier{holidays: holidays}
	for _, rule := range rules {
		switch rule.Kind {
		case SurchargeKindNight:
			c.night = append(c.night, rule)
		case SurchargeKindSunday:
			c.sunday = append(c.sunday, rule)
		case SurchargeKindHoliday:
			c.holiday = append(c.holiday, rule)
		}
	}
	return c
}

// add walks the span from boundary to boundary (midnight and the night
// windows' edges) and books each piece's seconds onto the rules covering it.
func (c *surchargeClassifier) add(span timeSpan, seconds map[int64]int64) {
	cursor := span.start
	for cursor.Before(span.end) {
		pieceEnd := minTime(c.nextBoundary(cursor), span.end)
		length := int64(pieceEnd.Sub(cursor) / time.Second)
		day := calendar.DateFromTime(cursor)
		if rule, ok := c.dayRule(day); ok {
			seconds[rule.ID] += length
		}
		if rule, ok := c.nightRule(wallMinute(cursor)); ok {
			seconds[rule.ID] += length
		}
		cursor = pieceEnd
	}
}

// nextBoundary is the first instant after t where the classification can
// change: the next midnight or the next edge of a night window.
func (c *surchargeClassifier) nextBoundary(t time.Time) time.Time {
	day := calendar.DateFromTime(t)
	next := day.AddDays(1).BerlinMidnight()
	for _, rule := range c.night {
		for _, minute := range []int{rule.WindowStartMinute, rule.WindowEndMinute} {
			edge := wallClockOn(day, minute)
			if edge.After(t) && edge.Before(next) {
				next = edge
			}
		}
	}
	return next
}

func (c *surchargeClassifier) dayRule(day calendar.Date) (PayrollSurchargeRule, bool) {
	var candidates []PayrollSurchargeRule
	if day.Weekday() == time.Sunday {
		candidates = append(candidates, c.sunday...)
	}
	if c.holidays[day] {
		candidates = append(candidates, c.holiday...)
	}
	return higherSurchargeRule(candidates)
}

func (c *surchargeClassifier) nightRule(minute int) (PayrollSurchargeRule, bool) {
	var candidates []PayrollSurchargeRule
	for _, rule := range c.night {
		if inNightWindow(minute, rule.WindowStartMinute, rule.WindowEndMinute) {
			candidates = append(candidates, rule)
		}
	}
	return higherSurchargeRule(candidates)
}

// higherSurchargeRule picks the rule that applies when several cover one
// minute: the higher rate of the same rate type; across rate types the holiday
// rule before the Sunday rule and the percent rule before the amount rule; the
// older rule on a tie.
func higherSurchargeRule(candidates []PayrollSurchargeRule) (PayrollSurchargeRule, bool) {
	if len(candidates) == 0 {
		return PayrollSurchargeRule{}, false
	}
	best := candidates[0]
	for _, rule := range candidates[1:] {
		if surchargeRuleBeats(rule, best) {
			best = rule
		}
	}
	return best, true
}

func surchargeRuleBeats(a, b PayrollSurchargeRule) bool {
	if a.RateType == b.RateType {
		if a.RateHundredths != b.RateHundredths {
			return a.RateHundredths > b.RateHundredths
		}
		return a.ID < b.ID
	}
	if a.Kind != b.Kind {
		return a.Kind == SurchargeKindHoliday
	}
	return a.RateType == SurchargeRatePercent
}

// inNightWindow reports whether a wall-clock minute lies in [start, end); a
// window with end before start runs across midnight.
func inNightWindow(minute, start, end int) bool {
	if start == end {
		return false
	}
	if start < end {
		return minute >= start && minute < end
	}
	return minute >= start || minute < end
}

func wallMinute(t time.Time) int {
	berlin := t.In(calendar.Berlin)
	return berlin.Hour()*60 + berlin.Minute()
}

func wallClockOn(day calendar.Date, minute int) time.Time {
	minute = ((minute % minutesPerDay) + minutesPerDay) % minutesPerDay
	return time.Date(day.Year(), day.Month(), day.Day(), minute/60, minute%60, 0, 0, calendar.Berlin)
}
