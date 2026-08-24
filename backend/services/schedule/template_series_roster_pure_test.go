// Pure unit tests for the decision helpers of the #2187 split-series roster
// reconciliation. They carry the branches the DB-backed tests cannot reach
// cheaply: which segments qualify, which weekdays a roster row covers, and
// when a kept row is already correct. No database, no fixtures — every value
// here is a stack-allocated struct.
package schedule

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/stretchr/testify/assert"
)

func seriesDate(day int) timezone.Date {
	return timezone.NewDate(2026, 8, day)
}

func datePtr(day int) *timezone.Date {
	d := seriesDate(day)
	return &d
}

func TestSegmentIsReconcilablePredecessor(t *testing.T) {
	t.Parallel()

	group := &activitiesModel.Group{}
	group.ID = 4711

	tests := []struct {
		name            string
		seg             templateSeriesSegment
		livingID        int64
		anchor          timezone.Date
		livingValidFrom *timezone.Date
		want            bool
	}{
		{
			name:     "bounded predecessor reaching past the anchor",
			seg:      templateSeriesSegment{Group: group, ValidFrom: datePtr(3), ValidUntil: datePtr(17)},
			livingID: 999,
			anchor:   seriesDate(10),
			want:     true,
		},
		{
			name:     "the living segment itself",
			seg:      templateSeriesSegment{Group: group, ValidFrom: datePtr(3), ValidUntil: datePtr(17)},
			livingID: group.ID,
			anchor:   seriesDate(10),
		},
		{
			name:     "still open, so not a predecessor",
			seg:      templateSeriesSegment{Group: group, ValidFrom: datePtr(3)},
			livingID: 999,
			anchor:   seriesDate(10),
		},
		{
			name:     "empty window left behind by a same-day split",
			seg:      templateSeriesSegment{Group: group, ValidFrom: datePtr(17), ValidUntil: datePtr(17)},
			livingID: 999,
			anchor:   seriesDate(10),
		},
		{
			name:     "ended before the change takes effect",
			seg:      templateSeriesSegment{Group: group, ValidFrom: datePtr(3), ValidUntil: datePtr(10)},
			livingID: 999,
			anchor:   seriesDate(10),
		},
		{
			name:            "starts at or after the living segment",
			seg:             templateSeriesSegment{Group: group, ValidFrom: datePtr(17), ValidUntil: datePtr(24)},
			livingID:        999,
			anchor:          seriesDate(10),
			livingValidFrom: datePtr(17),
		},
		{
			name:     "no group row",
			seg:      templateSeriesSegment{ValidFrom: datePtr(3), ValidUntil: datePtr(17)},
			livingID: 999,
			anchor:   seriesDate(10),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seg := tc.seg
			assert.Equal(t, tc.want,
				segmentIsReconcilablePredecessor(&seg, tc.livingID, tc.anchor, tc.livingValidFrom))
		})
	}
}

func TestCoveredSeriesWeekdays(t *testing.T) {
	t.Parallel()

	monday := activitiesModel.WeekdayMonday
	friday := activitiesModel.WeekdayFriday
	scoped := []int{activitiesModel.WeekdayMonday, activitiesModel.WeekdayWednesday}

	assert.Equal(t, []int{monday}, coveredSeriesWeekdays(&monday, scoped, scoped),
		"an explicit weekday inside the scope covers exactly itself")
	assert.Nil(t, coveredSeriesWeekdays(&friday, scoped, scoped),
		"an explicit weekday outside the scope covers nothing")
	assert.Equal(t, []int{monday}, coveredSeriesWeekdays(nil, []int{monday}, scoped),
		"a shared row covers its own universe intersected with the scope")
}

func TestWeekdayScopeReachesBeyondEdit(t *testing.T) {
	t.Parallel()

	segment := []int{activitiesModel.WeekdayMonday, activitiesModel.WeekdayWednesday}
	monday := activitiesModel.WeekdayMonday
	wednesday := activitiesModel.WeekdayWednesday

	full := weekdayScope{edited: segment, segment: segment}
	assert.False(t, full.reachesBeyondEdit(nil, segment, segment),
		"an edit describing every weekday can act on a shared row")

	narrow := weekdayScope{edited: []int{monday}, segment: segment}
	assert.True(t, narrow.reachesBeyondEdit(nil, segment, []int{monday}),
		"a shared row also covers the weekday this edit says nothing about")
	assert.False(t, narrow.reachesBeyondEdit(&monday, segment, []int{monday}),
		"a row explicit to the edited weekday covers nothing else")
	assert.True(t, narrow.reachesBeyondEdit(&wednesday, segment, nil),
		"and one explicit to another weekday is not this edit's business")
}

func TestWantedOnAllWeekdays(t *testing.T) {
	t.Parallel()

	const personID = 77
	want := map[seriesWeekdayPerson]bool{
		{weekday: activitiesModel.WeekdayMonday, personID: personID}: true,
	}

	assert.True(t, wantedOnAllWeekdays(want, []int{activitiesModel.WeekdayMonday}, personID, false))
	assert.False(t, wantedOnAllWeekdays(want,
		[]int{activitiesModel.WeekdayMonday, activitiesModel.WeekdayWednesday}, personID, false),
		"one uncovered weekday is enough to reject the row")
	assert.True(t, wantedOnAllWeekdays(want, nil, personID, true),
		"the caller decides what an empty weekday set means")
}

func TestPrimaryFlagMatches(t *testing.T) {
	t.Parallel()

	const personID = 77
	primaryWanted := map[seriesWeekdayPerson]bool{
		{weekday: activitiesModel.WeekdayMonday, personID: personID}: true,
	}
	weekdays := []int{activitiesModel.WeekdayMonday}

	assert.True(t, primaryFlagMatches(primaryWanted, weekdays, personID, true))
	assert.False(t, primaryFlagMatches(primaryWanted, weekdays, personID, false),
		"a supervisor who should be primary but is not needs rewriting")
	assert.False(t, primaryFlagMatches(primaryWanted,
		[]int{activitiesModel.WeekdayWednesday}, personID, true),
		"and so does one who is primary on a weekday that does not want it")
}

func TestSeriesRowStaysCorrect(t *testing.T) {
	t.Parallel()

	assert.True(t, seriesRowStaysCorrect(seriesDate(3), datePtr(17), seriesDate(10), datePtr(17)),
		"a row starting before the window and ending with the segment is already right")
	assert.False(t, seriesRowStaysCorrect(seriesDate(12), datePtr(17), seriesDate(10), datePtr(17)),
		"a row starting inside the window does not cover it")
	assert.False(t, seriesRowStaysCorrect(seriesDate(3), datePtr(12), seriesDate(10), datePtr(17)),
		"nor does one ending before the segment")
}

func TestProtectedPredecessorEnrollments(t *testing.T) {
	t.Parallel()

	requestChildID := int64(4242)
	protectedByRequest := &activitiesModel.StudentEnrollment{
		StudentID:                101,
		ValidFrom:                seriesDate(3),
		ValidUntil:               datePtr(17),
		EnrollmentRequestChildID: &requestChildID,
	}
	protectedButElsewhere := &activitiesModel.StudentEnrollment{
		StudentID:        102,
		ValidFrom:        seriesDate(24),
		ValidUntil:       datePtr(31),
		SelectedWeekdays: []int{activitiesModel.WeekdayMonday},
	}
	editorOwned := &activitiesModel.StudentEnrollment{
		StudentID:  103,
		ValidFrom:  seriesDate(3),
		ValidUntil: datePtr(17),
	}

	got := protectedPredecessorEnrollments(
		[]*activitiesModel.StudentEnrollment{nil, protectedByRequest, protectedButElsewhere, editorOwned},
		seriesDate(10), datePtr(17),
	)
	assert.Equal(t, []*activitiesModel.StudentEnrollment{protectedByRequest}, got,
		"only externally-owned rows overlapping the reconciliation window count")
}

func TestInt64SetDropsNonPositiveIDs(t *testing.T) {
	t.Parallel()

	set := int64Set([]int64{101, 0, -3, 101})
	assert.Len(t, set, 1)
	_, ok := set[101]
	assert.True(t, ok)
	assert.Empty(t, int64Set(nil), "no scope means nothing may be mirrored")
}
