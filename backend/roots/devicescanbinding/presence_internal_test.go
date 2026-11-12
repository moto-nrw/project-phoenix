package devicescanbinding

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/require"
)

type capacityPresence struct{ studentpresence.Presence }

func (capacityPresence) CreateVisit(context.Context, *studentpresence.Visit) error {
	return &studentpresence.ActivityParticipantLimitError{ActivityID: 9, ActivityName: "Hausaufgaben", CurrentOccupancy: 5, MaxParticipants: 5, Incoming: 1}
}

func TestVisitsRecordMapsLockedActivityCapacityRefusal(t *testing.T) {
	t.Parallel()
	recorder := visits{active: capacityPresence{}, clock: clock{now: func() time.Time { return time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC) }}}
	_, err := recorder.Record(t.Context(), 8, 7)
	capacity := activityCapacityExceeded(&studentpresence.ActivityParticipantLimitError{})
	require.ErrorAs(t, err, &capacity)
	require.Equal(t, int64(9), capacity.ActivityID)
	require.Equal(t, "Hausaufgaben", capacity.ActivityName)
	require.Equal(t, 5, capacity.CurrentOccupancy)
	require.Equal(t, 5, capacity.MaxCapacity)
	require.False(t, capacity.Details)
}
