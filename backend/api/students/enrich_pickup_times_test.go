package students

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

func TestApplyPickupTimesFromMap(t *testing.T) {
	t.Parallel()

	pickupAt := time.Date(2026, 4, 14, 14, 0, 0, 0, time.UTC)
	responses := []StudentResponse{
		{ID: 100, HasFullAccess: true},
		{ID: 200, HasFullAccess: false},
		{ID: 300, HasFullAccess: true},
	}
	pickupTimes := map[int64]*scheduleService.EffectivePickupTime{
		100: {
			PickupTime:  &pickupAt,
			IsException: true,
			Notes:       "Arzttermin",
			DayNotes: []scheduleService.NoteData{
				{ID: 1, Content: "Früher abholen"},
				{ID: 2, Content: ""},
			},
		},
		200: {PickupTime: &pickupAt},
		300: {IsException: true, Notes: "Ganztägig abwesend"},
	}

	applyPickupTimesFromMap(responses, pickupTimes)

	assert.Equal(t, "14:00", *responses[0].PickupTime)
	assert.True(t, responses[0].PickupIsException)
	assert.Equal(t, "Arzttermin, Früher abholen", responses[0].PickupNotes)
	assert.Nil(t, responses[1].PickupTime, "non-full-access student must not expose pickup data")
	assert.Nil(t, responses[2].PickupTime)
	assert.True(t, responses[2].PickupIsException)
	assert.Equal(t, "Ganztägig abwesend", responses[2].PickupNotes)
}

func TestApplyArrivalTimesFromMap(t *testing.T) {
	t.Parallel()

	arrivalAt := time.Date(2026, 4, 14, 8, 15, 0, 0, time.UTC)
	responses := []StudentResponse{
		{ID: 100, HasFullAccess: true},
		{ID: 200, HasFullAccess: false},
		{ID: 300, HasFullAccess: true},
	}
	arrivalTimes := map[int64]*scheduleService.EffectiveArrivalTime{
		100: {
			ArrivalTime: &arrivalAt,
			IsException: true,
			Notes:       "Später",
			DayNotes: []scheduleService.ArrivalNoteData{
				{ID: 11, Content: "Bitte anrufen"},
				{ID: 12, Content: ""},
			},
		},
		200: {ArrivalTime: &arrivalAt},
		300: {IsException: true, Notes: "Kommt heute nicht"},
	}

	applyArrivalTimesFromMap(responses, arrivalTimes)

	assert.Equal(t, "08:15", *responses[0].ArrivalTime)
	assert.True(t, responses[0].ArrivalIsException)
	assert.Equal(t, "Später, Bitte anrufen", responses[0].ArrivalNotes)
	assert.Nil(t, responses[1].ArrivalTime, "non-full-access student must not expose arrival data")
	assert.Nil(t, responses[2].ArrivalTime)
	assert.True(t, responses[2].ArrivalIsException)
	assert.Equal(t, "Kommt heute nicht", responses[2].ArrivalNotes)
}
