package timetable

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The patterns of #5343, anonymised from one school: a Randstunde offering
// (pickup 12:45, Monday to Friday) booked by Jahrgang 1/2 and 3/4. Jahrgang
// 1/2 sits in Klassen-Randstunden (11:45–12:45) on the two or three days its
// class arrives at 11:45; on the other days the class arrives at 12:45.
// Jahrgang 3/4 arrives at 12:45 or 13:30 every day. The lunch offering's
// Regeltermin starts at 12:45.

const gapMittag = int64(503)

const (
	clock1145 = 11*60 + 45
	clock1245 = 12*60 + 45
	clock1330 = 13*60 + 30
	clock1345 = 13*60 + 45
)

var everyPickup1245 = map[int]string{1: "12:45", 2: "12:45", 3: "12:45", 4: "12:45", 5: "12:45"}

// classDayBookings books Randstunde on the given weekdays for a school-day
// offering with its pickup times.
func classDayBookings(class string, firstStudent int64, count int, weekdays ...int) []OfferingWeekdayBooking {
	bookings := gapBookings(gapRandstunde, "Randstunde", class, firstStudent, count, weekdays...)
	for i := range bookings {
		bookings[i].ClassDayApplies = true
		bookings[i].PickupTimes = everyPickup1245
	}
	return bookings
}

func studentRange(first int64, count int) []int64 {
	ids := make([]int64, 0, count)
	for i := range count {
		ids = append(ids, first+int64(i))
	}
	return ids
}

// classTemplate is a Klasse Regeltermin without offering source whose
// target holds the children, at the given time on the given weekdays.
func classTemplate(students []int64, start, end int, weekdays ...int) OfferingGapTemplate {
	return OfferingGapTemplate{
		Occurrences: gapRuns(weekdays...), Kind: RosterOfferingCare, Times: clockTimes(start, end, weekdays...),
		Roster: &TemplatePlannedRoster{TargetStudentIDs: students},
	}
}

// clockTimes gives every occurrence of gapRuns(weekdays) the same window,
// as the projection reports it.
func clockTimes(start, end int, weekdays ...int) map[calendar.Date][]OccurrenceClock {
	times := map[calendar.Date][]OccurrenceClock{}
	for date := range gapRuns(weekdays...) {
		times[date] = []OccurrenceClock{{StartMinute: start, EndMinute: end}}
	}
	return times
}

// mittagTemplate is the lunch offering's Regeltermin from 12:45, which plans
// the given children by its source rule.
func mittagTemplate(students []int64, weekdays ...int) OfferingGapTemplate {
	template := classTemplate(students, clock1245, clock1345, weekdays...)
	template.HasOfferingSource = true
	template.Sources = OfferingSourceFilter{OfferingIDs: []int64{gapMittag}}
	template.Roster = &TemplatePlannedRoster{Entries: entriesFor(students)}
	return template
}

func entriesFor(students []int64) []TemplateRosterEntry {
	entries := make([]TemplateRosterEntry, 0, len(students))
	for _, studentID := range students {
		entries = append(entries, TemplateRosterEntry{StudentID: studentID})
	}
	return entries
}

func weeklyArrivals(byClass map[string]map[int]int) OfferingGapArrivals {
	return OfferingGapArrivals{Weekly: byClass, Dated: map[string]map[calendar.Date]int{}}
}

// schoolAArrivals: 1a arrives at 11:45 on Monday, Wednesday and Thursday,
// at 12:45 on Tuesday and Friday; 3a at 12:45, on Tuesday at 13:30.
func schoolAArrivals() OfferingGapArrivals {
	return weeklyArrivals(map[string]map[int]int{
		"1a": {1: clock1145, 2: clock1245, 3: clock1145, 4: clock1145, 5: clock1245},
		"3a": {1: clock1245, 2: clock1330, 3: clock1245, 4: clock1245, 5: clock1245},
	})
}

func withArrivals(arrivals OfferingGapArrivals) OfferingGapScope {
	scope := gapScope
	scope.Arrivals = arrivals
	return scope
}

// TestDetectOfferingGapsClassDaySchoolA is the school of #5343 in small: the
// Klassen-Randstunde covers Jahrgang 1/2 on the days its class needs it, the
// other days need no Randstunde, and Jahrgang 3/4 arrives at the pickup time
// or later. The lunch offering's Regeltermin at 12:45 changes nothing. The
// hint stays empty instead of counting every booked child.
func TestDetectOfferingGapsClassDaySchoolA(t *testing.T) {
	t.Parallel()
	class1a := studentRange(100, 4)
	class3a := studentRange(200, 3)
	templates := []OfferingGapTemplate{
		classTemplate(class1a, clock1145, clock1245, 1, 3, 4),
		mittagTemplate(append(append([]int64{}, class1a...), class3a...), 1, 2, 3, 4, 5),
	}
	bookings := append(classDayBookings("1a", 100, 4, 1, 2, 3, 4, 5), classDayBookings("3a", 200, 3, 1, 2, 3, 4, 5)...)

	assert.Empty(t, DetectOfferingGaps(withArrivals(schoolAArrivals()), templates, bookings))
}

// TestDetectOfferingGapsClassDayCoverage pins when a booked date with known
// clock times counts as covered.
func TestDetectOfferingGapsClassDayCoverage(t *testing.T) {
	t.Parallel()
	child := studentRange(300, 1)
	arrives1145 := weeklyArrivals(map[string]map[int]int{"2b": {1: clock1145, 3: clock1145}})
	handList := OfferingGapTemplate{
		Occurrences: gapRuns(1, 3), Kind: RosterOfferingCare,
		Times:  clockTimes(clock1145, clock1245, 1, 3),
		Roster: &TemplatePlannedRoster{Entries: entriesFor(child)},
	}
	cases := []struct {
		name      string
		arrivals  OfferingGapArrivals
		templates []OfferingGapTemplate
		uncovered []int
	}{
		{name: "Klassen-Randstunde at the time", arrivals: arrives1145, templates: []OfferingGapTemplate{classTemplate(child, clock1145, clock1245, 1, 3)}},
		{name: "hand-kept list at the time", arrivals: arrives1145, templates: []OfferingGapTemplate{handList}},
		{name: "a partial overlap leaves a gap", arrivals: arrives1145, templates: []OfferingGapTemplate{classTemplate(child, 12*60, 13*60, 1, 3)}, uncovered: []int{1, 3}},
		{name: "another offering's Regeltermin over the whole need counts", arrivals: arrives1145, templates: []OfferingGapTemplate{
			func() OfferingGapTemplate {
				template := mittagTemplate(child, 1, 3)
				template.Times = clockTimes(11*60+30, clock1345, 1, 3)
				return template
			}(),
		}},
		{name: "another offering's Regeltermin from the pickup on does not", arrivals: arrives1145, templates: []OfferingGapTemplate{mittagTemplate(child, 1, 3)}, uncovered: []int{1, 3}},
		{name: "a Regeltermin ending at the arrival does not", arrivals: arrives1145, templates: []OfferingGapTemplate{classTemplate(child, 10*60, clock1145, 1, 3)}, uncovered: []int{1, 3}},
		{name: "a class Regeltermin without the child does not", arrivals: arrives1145, templates: []OfferingGapTemplate{classTemplate(studentRange(900, 2), clock1145, clock1245, 1, 3)}, uncovered: []int{1, 3}},
		{name: "only Monday planned", arrivals: arrives1145, templates: []OfferingGapTemplate{classTemplate(child, clock1145, clock1245, 1)}, uncovered: []int{3}},
		{name: "arrival at the pickup needs nothing", arrivals: weeklyArrivals(map[string]map[int]int{"2b": {1: clock1245, 3: clock1330}})},
		{name: "arrival one minute before the pickup needs care", arrivals: weeklyArrivals(map[string]map[int]int{"2b": {1: clock1245 - 1, 3: clock1245}}), uncovered: []int{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gaps := DetectOfferingGaps(withArrivals(tc.arrivals), tc.templates, classDayBookings("2b", 300, 1, 1, 3))
			if tc.uncovered == nil {
				assert.Empty(t, gaps)
				return
			}
			require.Len(t, gaps, 1)
			assert.Equal(t, tc.uncovered, gaps[0].Weekdays)
		})
	}
}

// TestDetectOfferingGapsStrictDayRuleWithoutTimes: without the class's
// arrival or the offering's pickup, and for offerings outside school days,
// only a Regeltermin without offering source and of the offering's kind that
// plans the child on that date covers it (rule B of #5343). The Klassen-
// Randstunde covers its own days only; the other days stay visible.
func TestDetectOfferingGapsStrictDayRuleWithoutTimes(t *testing.T) {
	t.Parallel()
	class1a := studentRange(100, 4)
	klassenRandstunde := classTemplate(class1a, clock1145, clock1245, 1, 3, 4)
	noPickup := func() []OfferingWeekdayBooking {
		bookings := classDayBookings("1a", 100, 4, 1, 2, 3, 4, 5)
		for i := range bookings {
			bookings[i].PickupTimes = nil
		}
		return bookings
	}
	holiday := func() []OfferingWeekdayBooking {
		bookings := classDayBookings("1a", 100, 4, 1, 2, 3, 4, 5)
		for i := range bookings {
			bookings[i].ClassDayApplies = false
		}
		return bookings
	}
	cases := []struct {
		name      string
		scope     OfferingGapScope
		bookings  []OfferingWeekdayBooking
		templates []OfferingGapTemplate
		uncovered []int
	}{
		{name: "no pickup times", scope: withArrivals(schoolAArrivals()), bookings: noPickup(), templates: []OfferingGapTemplate{klassenRandstunde}, uncovered: []int{2, 5}},
		{name: "no Klassentag", scope: gapScope, bookings: classDayBookings("1a", 100, 4, 1, 2, 3, 4, 5), templates: []OfferingGapTemplate{klassenRandstunde}, uncovered: []int{2, 5}},
		{name: "Klassentag of another class only", scope: withArrivals(weeklyArrivals(map[string]map[int]int{"3a": {1: clock1245}})), bookings: classDayBookings("1a", 100, 4, 1, 2, 3, 4, 5), templates: []OfferingGapTemplate{klassenRandstunde}, uncovered: []int{2, 5}},
		{name: "Ferienangebot keeps the strict rule despite the Klassentag", scope: withArrivals(schoolAArrivals()), bookings: holiday(), templates: []OfferingGapTemplate{klassenRandstunde}, uncovered: []int{2, 5}},
		{name: "Ferienangebot without any Regeltermin", scope: withArrivals(schoolAArrivals()), bookings: holiday(), uncovered: []int{1, 2, 3, 4, 5}},
		{name: "another offering's Regeltermin never covers", scope: gapScope, bookings: noPickup(), templates: []OfferingGapTemplate{mittagTemplate(class1a, 1, 2, 3, 4, 5)}, uncovered: []int{1, 2, 3, 4, 5}},
		{name: "an AG does not stand in for care", scope: gapScope, bookings: noPickup(), templates: []OfferingGapTemplate{
			func() OfferingGapTemplate {
				template := classTemplate(class1a, clock1145, clock1245, 1, 2, 3, 4, 5)
				template.Kind = RosterOfferingCourse
				return template
			}(),
		}, uncovered: []int{1, 2, 3, 4, 5}},
		{name: "an external Regeltermin of no kind does not stand in", scope: gapScope, bookings: noPickup(), templates: []OfferingGapTemplate{
			func() OfferingGapTemplate {
				template := classTemplate(class1a, clock1145, clock1245, 1, 2, 3, 4, 5)
				template.Kind = RosterOfferingAny
				return template
			}(),
		}, uncovered: []int{1, 2, 3, 4, 5}},
		{name: "a Regeltermin of a switched-off offering does not stand in", scope: gapScope, bookings: noPickup(), templates: []OfferingGapTemplate{
			func() OfferingGapTemplate {
				template := classTemplate(class1a, clock1145, clock1245, 1, 2, 3, 4, 5)
				template.HasOfferingSource = true
				return template
			}(),
		}, uncovered: []int{1, 2, 3, 4, 5}},
		{name: "a roster not read covers nothing", scope: gapScope, bookings: noPickup(), templates: []OfferingGapTemplate{
			func() OfferingGapTemplate {
				template := classTemplate(class1a, clock1145, clock1245, 1, 2, 3, 4, 5)
				template.Roster = nil
				return template
			}(),
		}, uncovered: []int{1, 2, 3, 4, 5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gaps := DetectOfferingGaps(tc.scope, tc.templates, tc.bookings)
			require.Len(t, gaps, 1)
			assert.Equal(t, tc.uncovered, gaps[0].Weekdays)
			assert.Equal(t, 4, gaps[0].StudentCount)
		})
	}
}

// TestDetectOfferingGapsLunchByMensaWithoutTimes: under the strict day rule
// a Mensa list stands in for a lunch offering, a care block for a care
// offering that includes lunch.
func TestDetectOfferingGapsLunchByMensaWithoutTimes(t *testing.T) {
	t.Parallel()
	child := studentRange(400, 2)
	mensa := classTemplate(child, 12*60, 12*60+30, 1)
	mensa.Kind = RosterOfferingLunch
	lunchOnly := gapBookings(gapMittag, "Mittag", "2a", 400, 2, 1)
	for i := range lunchOnly {
		lunchOnly[i].CountsAsCare, lunchOnly[i].IncludesLunch = false, true
	}
	assert.Empty(t, DetectOfferingGaps(gapScope, []OfferingGapTemplate{mensa}, lunchOnly))
	assert.Len(t, DetectOfferingGaps(gapScope, []OfferingGapTemplate{classTemplate(child, 12*60, 13*60, 1)}, lunchOnly), 1,
		"a care block does not stand in for a lunch-only offering")
}

// TestDetectOfferingGapsRosterRowsByDate: a hand-kept row covers the dates
// it applies on, as materialization fills the occurrence: its validity, its
// planning period and its weekdays.
func TestDetectOfferingGapsRosterRowsByDate(t *testing.T) {
	t.Parallel()
	may := calendar.NewDate(2030, 5, 1)
	otherPeriod := int64(77)
	monday := WeekdayMonday
	cases := []struct {
		name    string
		entry   TemplateRosterEntry
		wantGap bool
	}{
		{name: "open row", entry: TemplateRosterEntry{StudentID: 500}},
		{name: "row ending before the end of scope", entry: TemplateRosterEntry{StudentID: 500, ValidUntil: &may}, wantGap: true},
		{name: "row starting after the scope start", entry: TemplateRosterEntry{StudentID: 500, ValidFrom: &may}, wantGap: true},
		{name: "row of another planning period", entry: TemplateRosterEntry{StudentID: 500, CalendarPeriodID: &otherPeriod}, wantGap: true},
		{name: "row of the planning period", entry: TemplateRosterEntry{StudentID: 500, CalendarPeriodID: new(int64(9))}},
		{name: "row for Monday", entry: TemplateRosterEntry{StudentID: 500, Weekday: &monday}},
		{name: "row for Tuesday only", entry: TemplateRosterEntry{StudentID: 500, Weekday: new(2)}, wantGap: true},
		{name: "row selecting Wednesday", entry: TemplateRosterEntry{StudentID: 500, SelectedWeekdays: []int{3}}, wantGap: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			template := classTemplate(nil, clock1145, clock1245, 1)
			template.Roster = &TemplatePlannedRoster{Entries: []TemplateRosterEntry{tc.entry}}
			scope := gapScope
			scope.PeriodID = 9
			gaps := DetectOfferingGaps(scope, []OfferingGapTemplate{template}, gapBookings(gapRandstunde, "Randstunde", "2c", 500, 1, 1))
			assert.Equal(t, tc.wantGap, len(gaps) == 1, "%+v", gaps)
		})
	}
}

// TestDetectOfferingGapsDatedArrival: a class-wide change on one date wins
// over the weekly time; a Tuesday on which 3a leaves school at 11:45 needs
// the Randstunde.
func TestDetectOfferingGapsDatedArrival(t *testing.T) {
	t.Parallel()
	arrivals := schoolAArrivals()
	early := NextWeekdayOnOrAfter(gapScope.From, 2).AddDays(14)
	arrivals.Dated["3a"] = map[calendar.Date]int{early: clock1145}
	gaps := DetectOfferingGaps(withArrivals(arrivals), nil, classDayBookings("3a", 200, 3, 1, 2, 3, 4, 5))
	require.Len(t, gaps, 1)
	assert.Equal(t, []int{2}, gaps[0].Weekdays)
	assert.Equal(t, 3, gaps[0].StudentCount)

	arrivals.Dated["3a"] = map[calendar.Date]int{early: clock1330}
	assert.Empty(t, DetectOfferingGaps(withArrivals(arrivals), nil, classDayBookings("3a", 200, 3, 1, 2, 3, 4, 5)),
		"a later arrival on one date needs nothing either")
}

// TestDetectOfferingGapsOccurrenceTimes: the check reads each date's own
// clock windows, as the projection reports them. A single occurrence moved
// out of the need leaves that date open; a second occurrence that day
// counts only when it covers the need; a date without a known time covers
// nothing.
func TestDetectOfferingGapsOccurrenceTimes(t *testing.T) {
	t.Parallel()
	child := studentRange(600, 1)
	arrivals := weeklyArrivals(map[string]map[int]int{"2a": {1: clock1145}})
	bookings := func() []OfferingWeekdayBooking { return classDayBookings("2a", 600, 1, 1) }
	moved := NextWeekdayOnOrAfter(gapScope.From, 1).AddDays(21)

	template := classTemplate(child, clock1145, clock1245, 1)
	require.Empty(t, DetectOfferingGaps(withArrivals(arrivals), []OfferingGapTemplate{template}, bookings()))

	template.Times[moved] = []OccurrenceClock{{StartMinute: 14 * 60, EndMinute: 15 * 60}}
	gaps := DetectOfferingGaps(withArrivals(arrivals), []OfferingGapTemplate{template}, bookings())
	require.Len(t, gaps, 1, "the moved Monday is open")
	assert.Equal(t, []int{1}, gaps[0].Weekdays)

	template.Times[moved] = []OccurrenceClock{{StartMinute: 14 * 60, EndMinute: 15 * 60}, {StartMinute: 12 * 60, EndMinute: 13 * 60}}
	assert.Len(t, DetectOfferingGaps(withArrivals(arrivals), []OfferingGapTemplate{template}, bookings()), 1,
		"a second occurrence that only overlaps the need leaves it open")

	template.Times[moved] = []OccurrenceClock{{StartMinute: 14 * 60, EndMinute: 15 * 60}, {StartMinute: clock1145, EndMinute: clock1245}}
	assert.Empty(t, DetectOfferingGaps(withArrivals(arrivals), []OfferingGapTemplate{template}, bookings()),
		"a second occurrence that day covers the need")

	delete(template.Times, moved)
	assert.Len(t, DetectOfferingGaps(withArrivals(arrivals), []OfferingGapTemplate{template}, bookings()), 1,
		"an occurrence without a known time overlaps nothing")
}

// TestDetectOfferingGapsNeverAddsToTheSourceCheck: the offering's own
// Regeltermin covers a date whatever the clock says, so the check reports a
// subset of #5140's.
func TestDetectOfferingGapsNeverAddsToTheSourceCheck(t *testing.T) {
	t.Parallel()
	fed := OfferingGapTemplate{
		Occurrences: gapRuns(1, 2, 3, 4, 5), Sources: OfferingSourceFilter{OfferingIDs: []int64{gapRandstunde}}, HasOfferingSource: true,
		Times: clockTimes(15*60, 16*60, 1),
	}
	assert.Empty(t, DetectOfferingGaps(withArrivals(schoolAArrivals()), []OfferingGapTemplate{fed}, classDayBookings("1a", 100, 4, 1, 2, 3, 4, 5)))
}

// TestDetectOfferingGapsDecisionStaysValid: a decision stored while the
// hint named more classes still covers what remains, so existing
// acknowledgements stay valid when Jahrgang 1/2 drops out.
func TestDetectOfferingGapsDecisionStaysValid(t *testing.T) {
	t.Parallel()
	arrivals := schoolAArrivals()
	arrivals.Weekly["3a"] = map[int]int{1: clock1145}
	bookings := withAck(append(classDayBookings("1a", 100, 4, 1, 3), classDayBookings("3a", 200, 3, 1)...), "1a", "3a")
	gaps := DetectOfferingGaps(withArrivals(arrivals), []OfferingGapTemplate{classTemplate(studentRange(100, 4), clock1145, clock1245, 1, 3)}, bookings)
	require.Len(t, gaps, 1)
	assert.Equal(t, []string{"3a"}, gaps[0].SchoolClasses)
	assert.True(t, gaps[0].Acknowledged)
	assert.Equal(t, []string{"1a", "3a"}, gaps[0].AcknowledgedClasses)
}

// TestDetectOfferingGapsNewSeries pins the Klassenfilter of a new
// Regeltermin started from the hint and whether it would plan children who
// need none (#5343): a second list for a class planned already.
func TestDetectOfferingGapsNewSeries(t *testing.T) {
	t.Parallel()
	class1a := studentRange(100, 4)
	arrivals := weeklyArrivals(map[string]map[int]int{
		"1a": {1: clock1145, 2: clock1145, 3: clock1145},
		"1b": {1: clock1145, 2: clock1245, 3: clock1145},
	})
	only1a := DetectOfferingGaps(withArrivals(arrivals), []OfferingGapTemplate{classTemplate(class1a, clock1145, clock1245, 1, 3)},
		classDayBookings("1a", 100, 4, 1, 2, 3))
	require.Len(t, only1a, 1)
	assert.Equal(t, []int{2}, only1a[0].Weekdays)
	assert.Equal(t, []string{"1a"}, only1a[0].NewSeriesClasses)
	assert.False(t, only1a[0].NewSeriesPlansCovered, "on Tuesday nobody of 1a is planned")

	mixed := DetectOfferingGaps(withArrivals(arrivals), []OfferingGapTemplate{classTemplate(class1a, clock1145, clock1245, 1, 3)},
		append(classDayBookings("1a", 100, 4, 1, 2, 3), classDayBookings("1b", 300, 2, 1, 2, 3)...))
	require.Len(t, mixed, 1)
	assert.Equal(t, []int{1, 2, 3}, mixed[0].Weekdays)
	assert.Equal(t, []string{"1a", "1b"}, mixed[0].NewSeriesClasses)
	assert.True(t, mixed[0].NewSeriesPlansCovered, "Monday and Wednesday would put 1a on a second list")

	withoutClass := DetectOfferingGaps(gapScope, nil, append(gapBookings(gapRandstunde, "Randstunde", "", 700, 1, 1), gapBookings(gapRandstunde, "Randstunde", "3a", 710, 1, 1)...))
	require.Len(t, withoutClass, 1)
	assert.Empty(t, withoutClass[0].NewSeriesClasses, "a Klassenfilter would leave the child without a class out")
	assert.False(t, withoutClass[0].NewSeriesPlansCovered)

	fedForOthers := DetectOfferingGaps(gapScope, []OfferingGapTemplate{
		{Occurrences: gapRuns(1), Sources: OfferingSourceFilter{OfferingIDs: []int64{gapRandstunde}, GradeLevels: []int{1}}, HasOfferingSource: true},
	}, append(gapBookings(gapRandstunde, "Randstunde", "", 700, 1, 1), gapBookings(gapRandstunde, "Randstunde", "1a", 720, 1, 1)...))
	require.Len(t, fedForOthers, 1)
	assert.True(t, fedForOthers[0].NewSeriesPlansCovered, "without a Klassenfilter the new Regeltermin would take 1a a second time")
}

func TestParseOfferingGapClock(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]int{"12:45": clock1245, " 07:05 ": 7*60 + 5, "13:30:00": clock1330, "00:00": 0} {
		got, ok := ParseOfferingGapClock(value)
		assert.True(t, ok, value)
		assert.Equal(t, want, got, value)
	}
	for _, value := range []string{"", "12", "24:00", "12:60", "ab:cd", "1:2:3:4"} {
		_, ok := ParseOfferingGapClock(value)
		assert.False(t, ok, value)
	}
}

// TestDetectOfferingGapsGaplessCoverage pins the decision on #5343: a day
// counts as planned only when the Regeltermine that plan the child that day
// cover the need from arrival to pickup together, without a gap. Windows are
// half-open wall-clock minutes, so occurrences that meet are seamless.
func TestDetectOfferingGapsGaplessCoverage(t *testing.T) {
	t.Parallel()
	child := studentRange(700, 1)
	arrives1145 := weeklyArrivals(map[string]map[int]int{"2a": {1: clock1145}})
	pickup1600 := func() []OfferingWeekdayBooking {
		bookings := classDayBookings("2a", 700, 1, 1)
		bookings[0].PickupTimes = map[int]string{1: "16:00"}
		return bookings
	}
	window := func(start, end int) OfferingGapTemplate { return classTemplate(child, start, end, 1) }
	cases := []struct {
		name      string
		templates []OfferingGapTemplate
		wantGap   bool
	}{
		{name: "Lernzeit 13–14 bei Bedarf 11:45–16:00 → Lücke", templates: []OfferingGapTemplate{window(13*60, 14*60)}, wantGap: true},
		{name: "Randstunde 11:45–12:45 und Ganztag 12:45–16:00 aneinander → gedeckt", templates: []OfferingGapTemplate{window(clock1145, clock1245), window(clock1245, 16*60)}},
		{name: "one Regeltermin over the whole stretch", templates: []OfferingGapTemplate{window(clock1145, 16*60)}},
		{name: "a wider Regeltermin", templates: []OfferingGapTemplate{window(11*60, 17*60)}},
		{name: "overlapping Regeltermine", templates: []OfferingGapTemplate{window(11*60, 13*60), window(12*60+30, 16*60+30)}},
		{name: "one minute missing between them", templates: []OfferingGapTemplate{window(clock1145, clock1245-1), window(clock1245, 16*60)}, wantGap: true},
		{name: "the start is missing", templates: []OfferingGapTemplate{window(12*60, 16*60)}, wantGap: true},
		{name: "the end is missing", templates: []OfferingGapTemplate{window(clock1145, 15*60+59)}, wantGap: true},
		{name: "a Regeltermin without the child does not fill the gap", templates: []OfferingGapTemplate{
			window(clock1145, clock1245), classTemplate(studentRange(990, 1), clock1245, 16*60, 1),
		}, wantGap: true},
		{name: "another offering's Regeltermin fills its part", templates: []OfferingGapTemplate{
			window(clock1145, clock1245),
			func() OfferingGapTemplate {
				template := mittagTemplate(child, 1)
				template.Times = clockTimes(clock1245, 16*60, 1)
				return template
			}(),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gaps := DetectOfferingGaps(withArrivals(arrives1145), tc.templates, pickup1600())
			assert.Equal(t, tc.wantGap, len(gaps) == 1, "%+v", gaps)
		})
	}
}

// TestDetectOfferingGapsGaplessPerOccurrence: two occurrences of one
// Regeltermin on a date add up like two Regeltermine.
func TestDetectOfferingGapsGaplessPerOccurrence(t *testing.T) {
	t.Parallel()
	child := studentRange(710, 1)
	arrivals := weeklyArrivals(map[string]map[int]int{"2b": {1: clock1145}})
	bookings := classDayBookings("2b", 710, 1, 1)
	bookings[0].PickupTimes = map[int]string{1: "16:00"}
	template := classTemplate(child, clock1145, clock1245, 1)
	for date := range template.Times {
		template.Times[date] = append(template.Times[date], OccurrenceClock{StartMinute: clock1245, EndMinute: 16 * 60})
	}
	assert.Empty(t, DetectOfferingGaps(withArrivals(arrivals), []OfferingGapTemplate{template}, bookings))
}
