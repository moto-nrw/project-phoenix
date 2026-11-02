package timetable

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	gapRandstunde = int64(501)
	gapGanztag    = int64(502)
)

// gapScope is a school year from a Monday on; gapRuns occur on every
// listed weekday of it.
var gapScope = OfferingGapScope{From: calendar.NewDate(2030, 3, 4), Until: calendar.NewDate(2030, 7, 31)}

func gapRuns(weekdays ...int) map[calendar.Date]bool {
	occurrences := map[calendar.Date]bool{}
	for _, weekday := range weekdays {
		for date := NextWeekdayOnOrAfter(gapScope.From, weekday); !date.After(gapScope.Until); date = date.AddDays(7) {
			occurrences[date] = true
		}
	}
	return occurrences
}

func gapBookings(offeringID int64, name, class string, firstStudent int64, count int, weekdays ...int) []OfferingWeekdayBooking {
	bookings := make([]OfferingWeekdayBooking, 0, count)
	for i := range count {
		bookings = append(bookings, OfferingWeekdayBooking{
			OfferingID: offeringID, OfferingName: name, StudentID: firstStudent + int64(i),
			SchoolClass: class, GradeLevel: gradeOf(class), Weekdays: weekdays, CountsAsCare: true,
		})
	}
	return bookings
}

func gradeOf(class string) *int {
	if class == "" {
		return nil
	}
	grade := int(class[0] - '0')
	return &grade
}

func withAck(bookings []OfferingWeekdayBooking, classes ...string) []OfferingWeekdayBooking {
	for i := range bookings {
		bookings[i].TemplateGapAckClasses = classes
	}
	return bookings
}

// TestDetectOfferingGapsSchuleAmBerg: Randstunde has a Regeltermin for
// Jahrgang 1 and 2 only; the booked children of Klasse 3 and 4 have none.
// Ganztag runs through its own link on every school day.
func TestDetectOfferingGapsSchuleAmBerg(t *testing.T) {
	t.Parallel()
	templates := []OfferingGapTemplate{
		{Occurrences: gapRuns(1, 2, 3, 4, 5), Sources: OfferingSourceFilter{OfferingIDs: []int64{gapRandstunde}, GradeLevels: []int{1, 2}}},
		{Occurrences: gapRuns(1, 2, 3, 4, 5), LinkedOfferingIDs: []int64{gapGanztag}},
	}
	var bookings []OfferingWeekdayBooking
	bookings = append(bookings, gapBookings(gapRandstunde, "Randstunde", "1a", 100, 4, 1, 2, 3, 4, 5)...)
	bookings = append(bookings, gapBookings(gapRandstunde, "Randstunde", "3a", 200, 8, 1, 2, 3, 4, 5)...)
	bookings = append(bookings, gapBookings(gapRandstunde, "Randstunde", "4c", 300, 5, 1, 3)...)
	bookings = append(bookings, gapBookings(gapGanztag, "Ganztag", "3a", 200, 8, 1, 2, 3, 4, 5)...)

	gaps := DetectOfferingGaps(gapScope, templates, bookings)

	require.Len(t, gaps, 1, "the Jahrgang 1/2 Regeltermin must not hide Klasse 3/4: %+v", gaps)
	gap := gaps[0]
	assert.Equal(t, gapRandstunde, gap.OfferingID)
	assert.Equal(t, "Randstunde", gap.OfferingName)
	assert.Equal(t, 13, gap.StudentCount)
	assert.Equal(t, []string{"3a", "4c"}, gap.SchoolClasses)
	assert.Equal(t, []int{1, 2, 3, 4, 5}, gap.Weekdays)
	assert.False(t, gap.Acknowledged)
}

// TestDetectOfferingGapsCoverage pins what counts as a matching
// Regeltermin.
func TestDetectOfferingGapsCoverage(t *testing.T) {
	t.Parallel()
	booking := gapBookings(gapRandstunde, "Randstunde", "3b", 400, 1, 1, 3)
	cases := []struct {
		name      string
		templates []OfferingGapTemplate
		uncovered []int
	}{
		{name: "source rule without filter", templates: []OfferingGapTemplate{
			{Occurrences: gapRuns(1, 3), Sources: OfferingSourceFilter{OfferingIDs: []int64{gapRandstunde}}},
		}},
		{name: "Klassenfilter of the class, any spelling", templates: []OfferingGapTemplate{
			{Occurrences: gapRuns(1, 3), Sources: OfferingSourceFilter{OfferingIDs: []int64{gapRandstunde}, SchoolClasses: []string{" 3B "}}},
		}},
		{name: "offering link", templates: []OfferingGapTemplate{
			{Occurrences: gapRuns(1, 3), LinkedOfferingIDs: []int64{gapRandstunde}},
		}},
		{name: "two Regeltermine share the days", templates: []OfferingGapTemplate{
			{Occurrences: gapRuns(1), Sources: OfferingSourceFilter{OfferingIDs: []int64{gapRandstunde}}},
			{Occurrences: gapRuns(3), LinkedOfferingIDs: []int64{gapRandstunde}},
		}},
		{name: "no Regeltermin", uncovered: []int{1, 3}},
		{name: "another offering's Regeltermin", templates: []OfferingGapTemplate{
			{Occurrences: gapRuns(1, 3), Sources: OfferingSourceFilter{OfferingIDs: []int64{gapGanztag}}},
		}, uncovered: []int{1, 3}},
		{name: "another class", templates: []OfferingGapTemplate{
			{Occurrences: gapRuns(1, 3), Sources: OfferingSourceFilter{OfferingIDs: []int64{gapRandstunde}, SchoolClasses: []string{"3a"}}},
		}, uncovered: []int{1, 3}},
		{name: "another Jahrgang", templates: []OfferingGapTemplate{
			{Occurrences: gapRuns(1, 3), Sources: OfferingSourceFilter{OfferingIDs: []int64{gapRandstunde}, GradeLevels: []int{1, 2}}},
		}, uncovered: []int{1, 3}},
		{name: "only Monday covered", templates: []OfferingGapTemplate{
			{Occurrences: gapRuns(1), Sources: OfferingSourceFilter{OfferingIDs: []int64{gapRandstunde}}},
		}, uncovered: []int{3}},
		{name: "ended series without running weekdays", templates: []OfferingGapTemplate{
			{Sources: OfferingSourceFilter{OfferingIDs: []int64{gapRandstunde}}},
		}, uncovered: []int{1, 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gaps := DetectOfferingGaps(gapScope, tc.templates, booking)
			if tc.uncovered == nil {
				assert.Empty(t, gaps)
				return
			}
			require.Len(t, gaps, 1)
			assert.Equal(t, tc.uncovered, gaps[0].Weekdays)
		})
	}
}

// TestDetectOfferingGapsSchoolDaysOnly: a booking without weekday
// restriction books every day; Saturday and Sunday never count.
func TestDetectOfferingGapsSchoolDaysOnly(t *testing.T) {
	t.Parallel()
	templates := []OfferingGapTemplate{{Occurrences: gapRuns(1, 2, 3, 4, 5), LinkedOfferingIDs: []int64{gapGanztag}}}
	assert.Empty(t, DetectOfferingGaps(gapScope, templates, gapBookings(gapGanztag, "Ganztag", "2a", 10, 3, 1, 2, 3, 4, 5, 6, 7)))
	assert.Empty(t, DetectOfferingGaps(gapScope, nil, gapBookings(gapGanztag, "Ganztag", "2a", 10, 3, 6, 7)))
}

// TestDetectOfferingGapsDecision pins when the school's decision hides the
// hint and when the hint comes back.
func TestDetectOfferingGapsDecision(t *testing.T) {
	t.Parallel()
	gap := func(bookings []OfferingWeekdayBooking) OfferingGap {
		t.Helper()
		gaps := DetectOfferingGaps(gapScope, nil, bookings)
		require.Len(t, gaps, 1)
		return gaps[0]
	}
	classes34 := func() []OfferingWeekdayBooking {
		return append(gapBookings(gapRandstunde, "Randstunde", "3a", 10, 2, 1), gapBookings(gapRandstunde, "Randstunde", "4a", 20, 2, 1)...)
	}

	t.Run("covers every listed class", func(t *testing.T) {
		t.Parallel()
		got := gap(withAck(classes34(), "3a", "4a"))
		assert.True(t, got.Acknowledged)
		assert.Equal(t, []string{"3a", "4a"}, got.AcknowledgedClasses)
	})
	t.Run("more children of a decided class stay hidden", func(t *testing.T) {
		t.Parallel()
		bookings := withAck(append(classes34(), gapBookings(gapRandstunde, "Randstunde", "3a", 30, 5, 1, 2)...), "3a", "4a")
		got := gap(bookings)
		assert.True(t, got.Acknowledged)
		assert.Equal(t, 9, got.StudentCount)
	})
	t.Run("a decided class that is covered now keeps the rest hidden", func(t *testing.T) {
		t.Parallel()
		assert.True(t, gap(withAck(gapBookings(gapRandstunde, "Randstunde", "4a", 20, 2, 1), "3a", "4a")).Acknowledged)
	})
	t.Run("a child of another class brings the hint back", func(t *testing.T) {
		t.Parallel()
		bookings := withAck(append(classes34(), gapBookings(gapRandstunde, "Randstunde", "4b", 40, 1, 1)...), "3a", "4a")
		got := gap(bookings)
		assert.False(t, got.Acknowledged)
		assert.Equal(t, []string{"3a", "4a", "4b"}, got.SchoolClasses)
		assert.Equal(t, []string{"3a", "4a"}, got.AcknowledgedClasses)
	})
	t.Run("children without a class need their own decision", func(t *testing.T) {
		t.Parallel()
		bookings := withAck(append(classes34(), gapBookings(gapRandstunde, "Randstunde", "", 50, 1, 1)...), "3a", "4a")
		assert.False(t, gap(bookings).Acknowledged)
		bookings = withAck(bookings, "", "3a", "4a")
		assert.True(t, gap(bookings).Acknowledged)
	})
	t.Run("no decision", func(t *testing.T) {
		t.Parallel()
		got := gap(classes34())
		assert.False(t, got.Acknowledged)
		assert.Empty(t, got.AcknowledgedClasses)
	})
	t.Run("a decision belongs to its offering", func(t *testing.T) {
		t.Parallel()
		bookings := append(withAck(classes34(), "3a", "4a"), gapBookings(gapGanztag, "Ganztag", "3a", 10, 2, 1)...)
		gaps := DetectOfferingGaps(gapScope, nil, bookings)
		require.Len(t, gaps, 2)
		assert.Equal(t, "Ganztag", gaps[0].OfferingName)
		assert.False(t, gaps[0].Acknowledged)
		assert.True(t, gaps[1].Acknowledged)
	})
}
