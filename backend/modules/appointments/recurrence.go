package appointments

import (
	"slices"
	"sort"
	"strings"
	"time"
)

func ExpandOccurrences(appointment *Appointment, rule *RecurrenceRule, from, to Date) []Date {
	occurrences := []Date{}
	appointmentStart := Date(appointment.StartDate)
	if rule.IntervalCount <= 0 {
		rule.IntervalCount = 1
	}
	span := appointment.StartDate.DaysUntil(appointment.EndDate)
	if rule.OccurrenceCount != nil {
		// Count-bounded: enumerate the first N occurrences by PERIOD (never a
		// day-by-day walk of decades of history for a sparse monthly/yearly series),
		// then keep those overlapping [from, to]. N is capped at
		// MaxRecurrenceOccurrenceCount during validation.
		for _, d := range boundedRecurrenceDates(appointment, rule, *rule.OccurrenceCount) {
			if dateRangesOverlap(d, d.AddDays(span), from, to) {
				occurrences = append(occurrences, d)
			}
		}
		return occurrences
	}
	// Open-ended or EndsOn-bounded: only the window matters, so anchor the scan at
	// the window (rewound by the appointment's own span to catch a multi-day
	// occurrence that begins before `from`), never before the series start, and
	// stop at `to` or EndsOn — whichever is earlier.
	scanStart := from.AddDays(-span)
	if scanStart.Before(appointmentStart) {
		scanStart = appointmentStart
	}
	scanEnd := to
	if rule.EndsOn != nil && Date(*rule.EndsOn).Before(scanEnd) {
		scanEnd = Date(*rule.EndsOn)
	}
	for d := scanStart; !d.After(scanEnd); d = d.AddDays(1) {
		if matchesRule(appointmentStart, d, rule) &&
			dateRangesOverlap(d, d.AddDays(span), from, to) {
			occurrences = append(occurrences, d)
		}
	}
	return occurrences
}

// boundedRecurrenceDates returns up to `limit` occurrence start-dates the rule
// generates on/after the appointment start, honouring EndsOn. It advances one
// interval-PERIOD per step (never day-by-day), so a sparse count-bounded
// monthly/yearly series with a large occurrence count is enumerated without
// scanning decades of history on every feed poll. Dates come back in
// chronological order — identical to the day-walk it replaces. Returns fewer
// than `limit` when the rule is exhausted (EndsOn reached, or no further
// reachable occurrence exists).
func boundedRecurrenceDates(appointment *Appointment, rule *RecurrenceRule, limit int) []Date {
	if rule == nil || limit <= 0 {
		return nil
	}
	interval := rule.IntervalCount
	if interval <= 0 {
		interval = 1
	}
	batch := boundedRecurrence{
		start: Date(appointment.StartDate), rule: rule,
		interval: interval, limit: limit, dates: make([]Date, 0, limit),
	}
	switch rule.Frequency {
	case RecurrenceFrequencyDaily:
		batch.daily()
	case RecurrenceFrequencyWeekly:
		batch.weekly()
	case RecurrenceFrequencyMonthly:
		batch.monthly()
	case RecurrenceFrequencyYearly:
		batch.yearly()
	}
	return batch.dates
}

// boundedRecurrence collects a frequency's chronological candidates within the
// series bounds. Each walker advances by interval periods rather than by days.
type boundedRecurrence struct {
	start    Date
	rule     *RecurrenceRule
	interval int
	limit    int
	dates    []Date
}

func (b *boundedRecurrence) full() bool {
	return len(b.dates) >= b.limit
}

func (b *boundedRecurrence) pastEnd(d Date) bool {
	return b.rule.EndsOn != nil && d.After(Date(*b.rule.EndsOn))
}

func (b *boundedRecurrence) appendCandidate(d Date) {
	if !d.Before(b.start) && !b.pastEnd(d) {
		b.dates = append(b.dates, d)
	}
}

func (b *boundedRecurrence) daily() {
	for k := 0; !b.full(); k++ {
		d := b.start.AddDays(k * b.interval)
		if b.pastEnd(d) {
			break
		}
		b.dates = append(b.dates, d)
	}
}

func (b *boundedRecurrence) weekly() {
	offsets := weekdayOffsets(normalizeRecurrenceWeekdays(b.rule.Weekdays))
	weekBase := weekStartMonday(b.start)
	// Without explicit weekdays, each interval lands on the start weekday.
	if len(offsets) == 0 {
		offsets = []int{(int(b.start.Weekday()) + 6) % 7}
	}
	for w := 0; !b.full(); w += b.interval {
		weekStart := weekBase.AddDays(w * 7)
		if b.pastEnd(weekStart) {
			break
		}
		for _, off := range offsets {
			if b.full() {
				break
			}
			b.appendCandidate(weekStart.AddDays(off))
		}
	}
}

func (b *boundedRecurrence) monthly() {
	days := monthlyCandidateDays(b.rule, b.start)
	// Reachable (month-of-year, leap-phase) patterns repeat within this bound;
	// consecutive empty months beyond it prove that the rule is exhausted.
	maxEmpty := 4*(12/gcdInt(b.interval, 12)) + 12
	empty := 0
	for k := 0; !b.full(); k++ {
		year, month := addMonthsTo(b.start.Year(), int(b.start.Month()), k*b.interval)
		if b.pastEnd(NewDate(year, time.Month(month), 1)) {
			break
		}
		before := len(b.dates)
		b.appendMonth(year, month, days)
		if len(b.dates) == before {
			empty++
			if empty > maxEmpty {
				break
			}
		} else {
			empty = 0
		}
	}
}

func (b *boundedRecurrence) appendMonth(year, month int, days []int) {
	dim := daysInMonth(year, month)
	for _, day := range days {
		if b.full() {
			break
		}
		if day < 1 || day > dim {
			continue
		}
		b.appendCandidate(NewDate(year, time.Month(month), day))
	}
}

func (b *boundedRecurrence) yearly() {
	// Only Feb 29 is sparse. The Gregorian leap cycle bounds empty steps even
	// when the interval skips every leap year.
	const maxEmpty = 400
	empty := 0
	for k := 0; !b.full(); k++ {
		year := b.start.Year() + k*b.interval
		if daysInMonth(year, int(b.start.Month())) < b.start.Day() {
			empty++
			if empty > maxEmpty {
				break
			}
			continue
		}
		d := NewDate(year, b.start.Month(), b.start.Day())
		if b.pastEnd(d) {
			break
		}
		b.dates = append(b.dates, d)
		empty = 0
	}
}

// monthlyCandidateDays returns the sorted day-of-month values a monthly rule can
// land on: its explicit month_days, or the start day when none are given (which
// is exactly what matchesRule compares against).
func monthlyCandidateDays(rule *RecurrenceRule, start Date) []int {
	if len(rule.MonthDays) == 0 {
		return []int{start.Day()}
	}
	// De-duplicate: a repeated month day would emit that occurrence twice and
	// exhaust occurrence_count early.
	seen := make(map[int]bool, len(rule.MonthDays))
	days := make([]int, 0, len(rule.MonthDays))
	for _, day := range rule.MonthDays {
		if seen[day] {
			continue
		}
		seen[day] = true
		days = append(days, day)
	}
	sort.Ints(days)
	return days
}

// weekdayOffsets converts normalized weekday names into sorted Monday-based
// offsets, dropping any unrecognised name. Validation guarantees valid names, so
// a non-empty weekly rule yields a non-empty, ascending result.
func weekdayOffsets(weekdays []string) []int {
	offsets := make([]int, 0, len(weekdays))
	for _, weekday := range weekdays {
		if off, ok := weekdayMondayOffset[weekday]; ok {
			offsets = append(offsets, off)
		}
	}
	sort.Ints(offsets)
	return offsets
}

// HasOccurrenceInWindow reports whether the recurrence produces at least one
// occurrence overlapping [from, to] WITHOUT walking every day since the series
// start. For an open-ended or EndsOn-bounded series it scans only the window
// (bounded by the window width plus the appointment's own multi-day span); for a
// count-bounded series it expands the bounded first-N occurrences. Used by the
// subscription feed, which is polled frequently for old series.
func HasOccurrenceInWindow(appointment *Appointment, rule *RecurrenceRule, from, to Date) bool {
	if rule == nil || to.Before(from) {
		return false
	}
	if rule.IntervalCount <= 0 {
		rule.IntervalCount = 1
	}
	if rule.OccurrenceCount != nil {
		// Bounded by the count: ExpandOccurrences enumerates the first N occurrences
		// by period (not day-by-day), and N is capped at MaxRecurrenceOccurrenceCount
		// during validation, so the scan can never walk unbounded history on a feed
		// poll even for a decades-old sparse monthly/yearly series.
		return len(ExpandOccurrences(appointment, rule, from, to)) > 0
	}
	// Scan only the window. Start early enough to catch a multi-day occurrence
	// that begins before `from` but reaches into it, but never before the series
	// start; stop at `to` or EndsOn, whichever is earlier.
	duration := appointment.StartDate.DaysUntil(appointment.EndDate)
	start := from.AddDays(-duration)
	appointmentStart := Date(appointment.StartDate)
	if start.Before(appointmentStart) {
		start = appointmentStart
	}
	end := to
	if rule.EndsOn != nil && Date(*rule.EndsOn).Before(end) {
		end = Date(*rule.EndsOn)
	}
	for d := start; !d.After(end); d = d.AddDays(1) {
		if matchesRule(appointmentStart, d, rule) &&
			dateRangesOverlap(d, d.AddDays(duration), from, to) {
			return true
		}
	}
	return false
}

// FirstRecurrenceOccurrence returns the first calendar date on or after the
// appointment's StartDate that satisfies the recurrence rule. ok=false means the
// rule has NO valid occurrence: either it is mathematically impossible (e.g. a
// monthly rule that only ever lands on February asking for day 31), or its first
// occurrence falls after EndsOn. The occurrence is computed analytically per
// frequency — no fixed scan horizon — so a valid but sparse rule (a large
// interval whose first match is many years out) is never mis-rejected nor
// mis-anchored to StartDate.
//
// For a weekly rule whose selected weekdays exclude StartDate's own weekday the
// first occurrence is later than StartDate; the in-app expansion
// (ExpandOccurrences) starts there, so ICS DTSTART must too.
func FirstRecurrenceOccurrence(appointment *Appointment, rule *RecurrenceRule) (Date, bool) {
	if rule == nil {
		return Date(appointment.StartDate), false
	}
	interval := rule.IntervalCount
	if interval <= 0 {
		interval = 1
	}
	first, exists := firstMatchingOccurrence(Date(appointment.StartDate), interval, rule)
	if !exists {
		return Date(appointment.StartDate), false
	}
	if rule.EndsOn != nil && first.After(Date(*rule.EndsOn)) {
		return first, false
	}
	return first, true
}

// firstMatchingOccurrence computes the first date on or after start that the rule
// matches, ignoring EndsOn. exists=false only for a genuinely impossible rule
// (monthly month_days that never fall in any reachable month). Daily, weekly
// without weekdays, monthly without month_days, and yearly always match start.
func firstMatchingOccurrence(start Date, interval int, rule *RecurrenceRule) (Date, bool) {
	switch rule.Frequency {
	case RecurrenceFrequencyDaily, RecurrenceFrequencyYearly:
		return start, true
	case RecurrenceFrequencyWeekly:
		weekdays := normalizeRecurrenceWeekdays(rule.Weekdays)
		if len(weekdays) == 0 {
			return start, true
		}
		offsets := weekdayOffsets(weekdays)
		if len(offsets) == 0 {
			return start, false
		}
		// Compute the first occurrence directly instead of scanning ~interval*7
		// days (which an arbitrarily large interval would blow up). The start's own
		// calendar week is interval-week 0 (always valid): the first occurrence is
		// its earliest selected weekday on/after start. If every selected weekday in
		// that week precedes start, the first occurrence is the earliest selected
		// weekday in the next valid week — interval calendar weeks later. This
		// matches matchesRule's calendar-week anchoring exactly.
		weekStart := weekStartMonday(start)
		for _, off := range offsets {
			d := weekStart.AddDays(off)
			if !d.Before(start) {
				return d, true
			}
		}
		return weekStart.AddDays(interval*7 + offsets[0]), true
	case RecurrenceFrequencyMonthly:
		if len(rule.MonthDays) == 0 {
			return start, true
		}
		return firstMonthlyOccurrence(start, interval, rule.MonthDays)
	default:
		return start, false
	}
}

// firstMonthlyOccurrence finds the first reachable month (start.Month() + k·interval)
// that contains a requested month-day valid for that month and on/after start.
// The (month-of-year, leap-phase) pattern of reachable months repeats within a
// bounded number of steps, so scanning that many steps proves impossibility
// without an unbounded loop.
func firstMonthlyOccurrence(start Date, interval int, monthDays []int) (Date, bool) {
	days := append([]int(nil), monthDays...)
	sort.Ints(days)
	// 12/gcd distinct months of the year are reached within 12/gcd steps; the ×4
	// (+buffer) covers up to four leap cycles for a February-only day-29 rule.
	maxSteps := 4*(12/gcdInt(interval, 12)) + 12
	for k := 0; k <= maxSteps; k++ {
		year, month := addMonthsTo(start.Year(), int(start.Month()), k*interval)
		dim := daysInMonth(year, month)
		for _, day := range days {
			if day < 1 || day > dim {
				continue
			}
			candidate := NewDate(year, time.Month(month), day)
			if candidate.Before(start) {
				continue
			}
			return candidate, true
		}
	}
	return start, false
}

// addMonthsTo returns the year and 1-based month reached by adding delta months.
func addMonthsTo(year, month, delta int) (int, int) {
	total := year*12 + (month - 1) + delta
	return total / 12, total%12 + 1
}

// daysInMonth returns the number of days in the given 1-based month, honouring
// leap years (day 0 of the next month is the last day of this one).
func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func gcdInt(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		a = -a
	}
	if a == 0 {
		return 1
	}
	return a
}

// weekStartMonday returns the Monday of the calendar week containing d. Used to
// anchor weekly-recurrence intervals to calendar weeks rather than to 7-day
// blocks measured from the appointment's start weekday.
func weekStartMonday(d Date) Date {
	return d.AddDays(-((int(d.Weekday()) + 6) % 7))
}

func matchesRule(start, candidate Date, rule *RecurrenceRule) bool {
	if candidate.Before(start) {
		return false
	}
	switch rule.Frequency {
	case RecurrenceFrequencyDaily:
		return start.DaysUntil(candidate)%rule.IntervalCount == 0
	case RecurrenceFrequencyWeekly:
		// Anchor the interval to calendar weeks (Mon–Sun), not to 7-day blocks
		// counted from the start weekday. Counting from the start day splits the
		// week on that weekday, so e.g. a biweekly Wed start with Mon+Wed would
		// wrongly include the Monday five days later (still "week 0") and skip
		// the Monday in the next valid calendar week.
		weeks := weekStartMonday(start).DaysUntil(weekStartMonday(candidate)) / 7
		if weeks%rule.IntervalCount != 0 {
			return false
		}
		weekdays := normalizeRecurrenceWeekdays(rule.Weekdays)
		if len(weekdays) == 0 {
			return candidate.Weekday() == start.Weekday()
		}
		return slices.Contains(weekdays, strings.ToLower(candidate.Weekday().String()))
	case RecurrenceFrequencyMonthly:
		months := (candidate.Year()-start.Year())*12 + int(candidate.Month()-start.Month())
		if months < 0 || months%rule.IntervalCount != 0 {
			return false
		}
		if len(rule.MonthDays) == 0 {
			return candidate.Day() == start.Day()
		}
		return slices.Contains(rule.MonthDays, candidate.Day())
	case RecurrenceFrequencyYearly:
		years := candidate.Year() - start.Year()
		return years >= 0 && years%rule.IntervalCount == 0 && candidate.Month() == start.Month() && candidate.Day() == start.Day()
	default:
		return false
	}
}

func normalizeRecurrenceWeekdays(days []string) []string {
	out := make([]string, 0, len(days))
	seen := make(map[string]bool, len(days))
	for _, day := range days {
		normalized := strings.ToLower(strings.TrimSpace(day))
		// De-duplicate: a repeated weekday would make the count-bounded expansion
		// emit that date twice and exhaust occurrence_count early.
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	return out
}

func dateRangesOverlap(aStart, aEnd, bStart, bEnd Date) bool {
	return !aEnd.Before(bStart) && !aStart.After(bEnd)
}

// OccurrenceExists reports whether occurrenceDate is one of the dates the
// recurrence generates. Membership is decided arithmetically via matchesRule
// (O(1): frequency/interval/weekday/month-day and occurrenceDate >= StartDate)
// plus the EndsOn bound, so cancelling a far-future date never walks every day
// since the series start. Only a count-bounded rule needs expansion, and that is
// bounded by the (small) occurrence count rather than the distance to the date.
func OccurrenceExists(appointment *Appointment, rule *RecurrenceRule, occurrenceDate Date) bool {
	if !matchesRule(Date(appointment.StartDate), occurrenceDate, rule) {
		return false
	}
	if rule.EndsOn != nil && occurrenceDate.After(Date(*rule.EndsOn)) {
		return false
	}
	if rule.OccurrenceCount == nil {
		return true
	}
	// Count-bounded: the date matches the pattern and lies within EndsOn, so the
	// only remaining question is whether it falls within the first N occurrences.
	// ExpandOccurrences enumerates those by period (never day-by-day), so this
	// stays cheap even for a far-future date on an old series.
	for _, occ := range ExpandOccurrences(appointment, rule, Date(appointment.StartDate), occurrenceDate) {
		if occ == occurrenceDate {
			return true
		}
	}
	return false
}

// weekdayMondayOffset maps a lowercase weekday name to its Monday-based offset
// (Monday=0 … Sunday=6), matching weekStartMonday's anchoring.
var weekdayMondayOffset = map[string]int{
	"monday":    0,
	"tuesday":   1,
	"wednesday": 2,
	"thursday":  3,
	"friday":    4,
	"saturday":  5,
	"sunday":    6,
}
