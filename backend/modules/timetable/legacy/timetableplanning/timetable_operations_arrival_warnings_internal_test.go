package timetableplanning

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendArrivalWarningsAddsTheClassExceptionLine(t *testing.T) {
	t.Parallel()

	inst := &scheduleModel.ActivityInstance{StartTime: mustClock(t, "12:45")}
	clock := mustClock(t, "12:45")
	warnings := map[int64][]OperationRosterWarning{}

	appendArrivalWarnings(warnings, map[int64]*careplan.EffectiveArrivalTime{
		1: {ArrivalTime: &clock, ClassException: &careplan.ClassArrivalExceptionInfo{
			SchoolClass: "4a", ArrivalTime: "12:45", Label: "Klasse 4a: Unterricht fällt aus",
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

	inst := &scheduleModel.ActivityInstance{StartTime: mustClock(t, "12:45")}
	late := mustClock(t, "13:30")
	warnings := map[int64][]OperationRosterWarning{}

	appendArrivalWarnings(warnings, map[int64]*careplan.EffectiveArrivalTime{
		1: {ArrivalTime: &late, ClassException: &careplan.ClassArrivalExceptionInfo{
			SchoolClass: "4a", ArrivalTime: "13:30", Label: "Klasse 4a: Wandertag",
		}},
	}, inst)

	require.Len(t, warnings[1], 2)
	assert.Equal(t, "arrival_after_slot_start", warnings[1][0].Kind)
	assert.Equal(t, "class_arrival_exception", warnings[1][1].Kind)
	assert.Equal(t, "Klasse 4a: Wandertag", warnings[1][1].Message,
		"the roster already shows 'Kommt um 13:30 Uhr' from the late-arrival warning")
}

func mustClock(t *testing.T, hhmm string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", hhmm)
	require.NoError(t, err)
	return timezone.NormalizeWallClock(parsed)
}
