package worktimemodels

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildFieldsRejectsMalformedWireValues(t *testing.T) {
	t.Parallel()

	badClock := "0830"
	for _, test := range []struct {
		name    string
		request ModelRequest
		reason  string
	}{
		{
			name:    "anchor is not a calendar day",
			request: ModelRequest{Name: "Vollzeit", RotationLength: 1, RotationAnchorDate: "01.06.2026"},
			reason:  "rotation_anchor_date must be YYYY-MM-DD",
		},
		{
			name: "start time is not a wall clock",
			request: ModelRequest{
				Name: "Vollzeit", RotationLength: 1, RotationAnchorDate: "2026-06-01",
				Entries: []EntryRequest{{WeekIndex: 0, DayOfWeek: 0, TargetMinutes: 300, StartTime: &badClock}},
			},
			reason: "start_time must be HH:MM",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := buildFields(test.request)
			require.Error(t, err)
			assert.Equal(t, test.reason, err.Error())
		})
	}
}

// A zero-minute slot is how the UI clears a day; it must never reach the
// capability as an entry, or the template would keep an empty weekday.
func TestBuildFieldsDropsZeroMinuteEntriesAndNormalizesClocks(t *testing.T) {
	t.Parallel()

	startTime := "08:30"
	fields, err := buildFields(ModelRequest{
		Name: "Teilzeit", RotationLength: 2, RotationAnchorDate: "2026-06-01",
		Entries: []EntryRequest{
			{WeekIndex: 0, DayOfWeek: 0, TargetMinutes: 300, StartTime: &startTime},
			{WeekIndex: 0, DayOfWeek: 1, TargetMinutes: 0},
		},
	})
	require.NoError(t, err)
	require.Len(t, fields.Entries, 1)
	assert.Equal(t, "08:30:00", fields.Entries[0].StartTime)
	assert.Equal(t, "2026-06-01", fields.RotationAnchorDate)
}

func TestToResponseSumsWeeklyTotalsPerRotationWeek(t *testing.T) {
	t.Parallel()

	response := toResponse(workforce.WorkTimeModel{
		ID: 7, Name: "A/B", RotationLength: 2, RotationAnchorDate: "2026-06-01",
		Entries: []workforce.WorkTimeModelEntry{
			{WeekIndex: 0, DayOfWeek: 0, TargetMinutes: 300, StartTime: "08:30:00"},
			{WeekIndex: 0, DayOfWeek: 1, TargetMinutes: 240},
			{WeekIndex: 1, DayOfWeek: 0, TargetMinutes: 180},
		},
	})

	assert.Equal(t, []int{540, 180}, response.WeeklyTotals)
	require.NotNil(t, response.Entries[0].StartTime)
	assert.Equal(t, "08:30", *response.Entries[0].StartTime)
	assert.Nil(t, response.Entries[1].StartTime)
}
