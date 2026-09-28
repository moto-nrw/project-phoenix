package compose

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendArrivalWarningsAddsTheClassExceptionLine(t *testing.T) {
	t.Parallel()

	inst := &scheduleModels.ActivityInstance{StartTime: normalizedClock(t, "12:45")}
	clock := normalizedClock(t, "12:45")
	warnings := map[int64][]timetable.OperationRosterWarning{}

	appendArrivalWarnings(warnings, map[int64]*ExpectedArrival{
		1: {ArrivalTime: &clock, ClassException: &ClassArrivalNotice{
			ArrivalTime: "12:45", Label: "Klasse 4a: Unterricht fällt aus",
		}},
		2: {ArrivalTime: &clock},
	}, inst)

	require.Len(t, warnings[1], 1)
	assert.Equal(t, "class_arrival_exception", warnings[1][0].Kind)
	assert.Equal(t, "Kommt heute um 12:45 Uhr (Klasse 4a: Unterricht fällt aus)", warnings[1][0].Message)
	require.NotNil(t, warnings[1][0].ExpectedArrival)
	assert.Equal(t, "12:45", *warnings[1][0].ExpectedArrival)
	assert.Empty(t, warnings[2], "a regular arrival at block start carries no line")
}

func TestAppendArrivalWarningsKeepsOnlyTheReasonWhenTheClassArrivesLate(t *testing.T) {
	t.Parallel()

	inst := &scheduleModels.ActivityInstance{StartTime: normalizedClock(t, "12:45")}
	late := normalizedClock(t, "13:30")
	warnings := map[int64][]timetable.OperationRosterWarning{}

	appendArrivalWarnings(warnings, map[int64]*ExpectedArrival{
		1: {ArrivalTime: &late, ClassException: &ClassArrivalNotice{
			ArrivalTime: "13:30", Label: "Klasse 4a: Wandertag",
		}},
	}, inst)

	require.Len(t, warnings[1], 2)
	assert.Equal(t, "arrival_after_slot_start", warnings[1][0].Kind)
	assert.Equal(t, "class_arrival_exception", warnings[1][1].Kind)
	assert.Equal(t, "Klasse 4a: Wandertag", warnings[1][1].Message,
		"the roster already shows 'Kommt um 13:30 Uhr' from the late-arrival warning")
}

func normalizedClock(t *testing.T, hhmm string) time.Time {
	t.Helper()
	return timezone.NormalizeWallClock(mustClock(t, hhmm))
}
