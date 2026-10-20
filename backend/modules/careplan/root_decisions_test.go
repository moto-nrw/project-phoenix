package careplan

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestStatusDayOverviewShowsActiveAndEndOfDayRowsInsideTheWindows(t *testing.T) {
	t.Parallel()
	from, until := calendar.Date("2026-09-01"), calendar.Date("2026-09-30")
	options := StatusDayOverviewOptions(StatusDayOverview{
		From: from, To: until, Status: StudentStatusDaySick,
		Children: []StatusDayWindow{{StudentID: 31, From: &from}, {StudentID: 32, Until: &until}},
		Page:     3, PageSize: 20,
	})
	require.NotNil(t, options.Filter)
	assert.Equal(t, 20, options.Limit)
	assert.Equal(t, 40, options.Offset)
	assert.Equal(t, []StudentScheduleQueryCondition{
		{Field: "date", Operator: ">=", Value: from},
		{Field: "date", Operator: "<=", Value: until},
		{Field: "status", Operator: "=", Value: StudentStatusDaySick},
	}, options.Filter.Conditions)
	require.Len(t, options.Filter.And, 2)
	windows, active := options.Filter.And[0], options.Filter.And[1]
	assert.Equal(t, []StudentScheduleQueryCondition{{Field: "student_id", Operator: "=", Value: int64(31)}, {Field: "date", Operator: ">=", Value: from}}, windows.Conditions)
	require.Len(t, windows.Or, 1)
	assert.Equal(t, []StudentScheduleQueryCondition{{Field: "student_id", Operator: "=", Value: int64(32)}, {Field: "date", Operator: "<=", Value: until}}, windows.Or[0].Conditions)
	assert.Equal(t, []StudentScheduleQueryCondition{{Field: "cleared_at", Operator: "IS NULL"}}, active.Conditions)
	assert.Equal(t, []StudentScheduleQueryCondition{{Field: "source", Operator: "=", Value: "end_of_day"}}, active.Or[0].Conditions)
}

func TestStatusDayOverviewWithoutChildrenMatchesNothing(t *testing.T) {
	t.Parallel()
	options := StatusDayOverviewOptions(StatusDayOverview{Page: 1, PageSize: 10})
	assert.Empty(t, StatusDayOverviewOptions(StatusDayOverview{Page: 1}).Filter.Conditions[2:], "no status filter without a status")
	assert.Equal(t, []StudentScheduleQueryCondition{{Field: "student_id", Operator: "=", Value: int64(-1)}}, options.Filter.And[0].Conditions)
}

func TestEndOfDayStatusFlagArchiveMapsTheLiveFlags(t *testing.T) {
	t.Parallel()
	fallback := time.Date(2026, 9, 21, 7, 0, 0, 0, time.UTC)
	sick, err := EndOfDayStatusFlagArchive("sick", "sick_since", "2026-09-21", fallback)
	require.NoError(t, err)
	assert.Equal(t, StatusFlagArchive{FlagColumn: "sick", SinceColumn: "sick_since", Status: "sick", Date: "2026-09-21", ReportedFallback: fallback, Source: "end_of_day"}, sick)
	excused, err := EndOfDayStatusFlagArchive("excused", "excused_since", "2026-09-21", fallback)
	require.NoError(t, err)
	assert.Equal(t, "excused", excused.Status)
	_, err = EndOfDayStatusFlagArchive("trip", "trip_since", "2026-09-21", fallback)
	require.EqualError(t, err, `unsupported status flag column "trip"`)
}

func TestParticipationBoundaryTakesTheEarlierEnd(t *testing.T) {
	t.Parallel()
	pending := Date("2026-09-15")
	later := Date("2026-10-15")
	boundary, found := ParticipationBoundary("2026-09-30", &pending)
	require.True(t, found)
	assert.Equal(t, calendar.Date("2026-09-15"), boundary)
	boundary, found = ParticipationBoundary("2026-09-30", &later)
	require.True(t, found)
	assert.Equal(t, calendar.Date("2026-10-01"), boundary, "the day after the enrolment end")
	boundary, found = ParticipationBoundary("", &pending)
	require.True(t, found)
	assert.Equal(t, calendar.Date("2026-09-15"), boundary)
	_, found = ParticipationBoundary("", nil)
	assert.False(t, found)
}

func TestClassKeyIgnoresCaseAndSpacing(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ClassKey("1a"), ClassKey(" 1A "))
}

func TestParseOfferingStaffValue(t *testing.T) {
	t.Parallel()
	from, selections, err := ParseOfferingStaffValue(map[string]any{
		"effective_from": "2026-10-01",
		"selections":     []any{map[string]any{"offering_id": float64(12), "selected_days": []any{"mon", "wed"}}},
	})
	require.NoError(t, err)
	assert.Equal(t, calendar.Date("2026-10-01"), from)
	assert.Equal(t, []OfferingChangeSelection{{OfferingID: 12, SelectedDays: []string{"mon", "wed"}}}, selections)
	for name, value := range map[string]any{
		"missing value":   nil,
		"missing date":    map[string]any{"selections": []any{}},
		"bad date":        map[string]any{"effective_from": "01.10.2026", "selections": []any{}},
		"no selections":   map[string]any{"effective_from": "2026-10-01"},
		"fractional id":   map[string]any{"effective_from": "2026-10-01", "selections": []any{map[string]any{"offering_id": 1.5}}},
		"non-string days": map[string]any{"effective_from": "2026-10-01", "selections": []any{map[string]any{"offering_id": float64(12), "selected_days": []any{3}}}},
	} {
		_, _, err := ParseOfferingStaffValue(value)
		require.ErrorIs(t, err, ErrOfferingChangeInvalid, name)
	}
}
